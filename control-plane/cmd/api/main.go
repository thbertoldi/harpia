package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/sdk/client"

	"github.com/harpia/control-plane/gen/harpia/agents/v1/agentsv1connect"
	"github.com/harpia/control-plane/gen/harpia/artifacts/v1/artifactsv1connect"
	"github.com/harpia/control-plane/gen/harpia/executors/v1/executorsv1connect"
	"github.com/harpia/control-plane/gen/harpia/feedback/v1/feedbackv1connect"
	"github.com/harpia/control-plane/gen/harpia/identity/v1/identityv1connect"
	"github.com/harpia/control-plane/gen/harpia/plans/v1/plansv1connect"
	"github.com/harpia/control-plane/gen/harpia/tasks/v1/tasksv1connect"
	"github.com/harpia/control-plane/internal/agents"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/cache"
	"github.com/harpia/control-plane/internal/config"
	"github.com/harpia/control-plane/internal/database"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/feedback"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/plans"
	"github.com/harpia/control-plane/internal/server"
	"github.com/harpia/control-plane/internal/storage"
	"github.com/harpia/control-plane/internal/tasks"
	"github.com/harpia/control-plane/internal/workflow"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx := context.Background()

	if os.Getenv("HARPIA_ROLE") == "worker" {
		runWorker(ctx, cfg)
		return
	}

	runAPI(ctx, cfg, logger)
}

func fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func runWorker(ctx context.Context, cfg *config.Config) {
	if cfg.TemporalHost == "" {
		fatal("HARPIA_ROLE=worker requires TEMPORAL_HOST to be set")
	}
	if cfg.DatabaseURL == "" {
		fatal("HARPIA_ROLE=worker requires DATABASE_URL to be set")
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("database connection failed", "error", err)
	}
	defer pool.Close()

	planRepo := plans.NewRepository(pool)
	executorRepo := executors.NewRepository(pool)
	planActivities := &workflow.PlanActivities{
		Runtime: plans.NewRuntimeRepository(planRepo, executorRepo),
	}

	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHost})
	if err != nil {
		fatal("temporal client dial failed", "error", err)
	}
	defer c.Close()

	if err := workflow.StartWorker(ctx, c, workflow.TaskQueueName, planActivities); err != nil {
		fatal("temporal worker failed", "error", err)
	}
}

type apiCacheResources struct {
	client               *cache.Client
	tenantStore          *cache.TenantStore
	agentCapabilityCache *cache.AgentCapabilityCache
	rateLimiter          *cache.RateLimiter
}

func setupAPICache(ctx context.Context, valkeyURL string, logger *slog.Logger) apiCacheResources {
	var resources apiCacheResources
	if valkeyURL == "" {
		return resources
	}

	cacheClient, err := cache.NewClient(valkeyURL)
	if err != nil {
		logger.Warn("valkey connection failed, running without cache", "error", err)
		return resources
	}

	if pingErr := cacheClient.Ping(ctx); pingErr != nil {
		logger.Warn("valkey ping failed, running without cache", "error", pingErr)
		return resources
	}

	resources.client = cacheClient
	resources.tenantStore = cache.NewTenantStore(cacheClient)
	resources.agentCapabilityCache = cache.NewAgentCapabilityCache(resources.tenantStore)
	resources.rateLimiter = cache.NewRateLimiter(resources.tenantStore)
	return resources
}

func setupTemporalClient(cfg *config.Config, logger *slog.Logger) *workflow.TemporalClient {
	if cfg.TemporalHost == "" {
		return nil
	}

	temporalClient, err := workflow.NewTemporalClient(cfg.TemporalHost)
	if err != nil {
		logger.Warn("temporal client failed, running without workflow engine", "error", err)
		return nil
	}
	return temporalClient
}

func runAPI(ctx context.Context, cfg *config.Config, logger *slog.Logger) {
	if cfg.DatabaseURL == "" {
		fatal("DATABASE_URL is required")
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("database connection failed", "error", err)
	}
	defer pool.Close()

	tenantID, userID, err := database.EnsureDevData(ctx, pool)
	if err != nil {
		fatal("seed dev data failed", "error", err)
	}
	logger.Info("dev data ensured", "tenant_id", tenantID.String(), "user_id", userID.String())

	if err := executors.EnsureCatalog(ctx, pool); err != nil {
		fatal("seed executor catalog failed", "error", err)
	}
	if err := executors.EnsureDevEntitlements(ctx, pool, tenantID); err != nil {
		fatal("seed dev executor entitlements failed", "error", err)
	}
	logger.Info("executor catalog and dev entitlements ensured")

	taskRepo := tasks.NewRepository(pool)
	planRepo := plans.NewRepository(pool)
	agentRepo := agents.NewRepository(pool)
	executorRepo := executors.NewRepository(pool)

	cacheResources := setupAPICache(ctx, cfg.ValkeyURL, logger)
	if cacheResources.client != nil {
		defer cacheResources.client.Close()
	}
	temporalClient := setupTemporalClient(cfg, logger)
	if temporalClient != nil {
		defer temporalClient.Close()
	}

	taskHandler, err := tasks.NewTaskHandler(taskRepo, temporalClient, cacheResources.tenantStore, userID)
	if err != nil {
		fatal("create task handler failed", "error", err)
	}

	agentHandler, err := agents.NewAgentHandler(agentRepo, agents.NewNoopEmbedder(), cacheResources.agentCapabilityCache)
	if err != nil {
		fatal("create agent handler failed", "error", err)
	}

	scheduleManager := plans.NewScheduleManager(temporalClient, logger)
	var planHandler *plans.PlanHandler
	if temporalClient != nil {
		planHandler, err = plans.NewPlanHandler(planRepo, executorRepo, scheduleManager, temporalClient)
	} else {
		planHandler, err = plans.NewPlanHandler(planRepo, executorRepo, scheduleManager)
	}
	if err != nil {
		fatal("create plan handler failed", "error", err)
	}

	executorHandler, err := executors.NewHandler(executorRepo)
	if err != nil {
		fatal("create executor handler failed", "error", err)
	}

	artifactRepo := artifacts.NewRepository(pool)
	garageStore, err := artifacts.NewGarageStore(artifacts.GarageConfig{
		Endpoint:  cfg.GarageURL,
		Bucket:    cfg.GarageBucket,
		Region:    cfg.GarageRegion,
		AccessKey: cfg.GarageAccessKey,
		SecretKey: cfg.GarageSecretKey,
	}, storage.NewTenantObjectStore(cfg.GarageBucket))
	if err != nil {
		fatal("create garage store failed", "error", err)
	}
	artifactHandler, err := artifacts.NewHandler(artifactRepo, garageStore)
	if err != nil {
		fatal("create artifact handler failed", "error", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"service":   "harpia-api",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	interceptors := []connect.Interceptor{identity.NewRequestContextInterceptor(identity.AuthOptions{
		DevTenantID:  tenantID,
		AllowDevAuth: cfg.AllowDevAuth,
		Authenticator: identity.NewZitadelAuthenticator(
			cfg.ZitadelURL,
			nil,
			cfg.AuthCacheTTL,
			cfg.AuthCacheMaxEntries,
		).WithHostHeader(cfg.ZitadelHost),
		Memberships: identity.NewMembershipRepository(pool, identity.MembershipRepositoryOptions{
			DefaultTenantID:            tenantID,
			AutoProvisionDefaultTenant: cfg.AutoProvisionDefaultTenant,
		}),
	})}
	if cacheResources.rateLimiter != nil {
		interceptors = append(interceptors, server.NewRateLimitInterceptor(cacheResources.rateLimiter, 100))
	}
	requestContext := connect.WithInterceptors(interceptors...)

	agentsPath, agentsHandler := agentsv1connect.NewAgentServiceHandler(agentHandler, requestContext)
	executorsPath, executorsHandler := executorsv1connect.NewExecutorServiceHandler(executorHandler, requestContext)
	tasksPath, tasksHandler := tasksv1connect.NewTaskServiceHandler(taskHandler, requestContext)
	plansPath, plansHandler := plansv1connect.NewPlanServiceHandler(planHandler, requestContext)
	artifactsPath, artifactsHandler := artifactsv1connect.NewArtifactServiceHandler(artifactHandler, requestContext)
	identityPath, identityHandler := identityv1connect.NewIdentityServiceHandler(identity.NewIdentityHandler(), requestContext)
	feedbackPath, feedbackHandler := feedbackv1connect.NewFeedbackServiceHandler(feedback.NewFeedbackHandler(), requestContext)

	mux.Handle(agentsPath, agentsHandler)
	mux.Handle(executorsPath, executorsHandler)
	mux.Handle(tasksPath, tasksHandler)
	mux.Handle(plansPath, plansHandler)
	mux.Handle(artifactsPath, artifactsHandler)
	mux.Handle(identityPath, identityHandler)
	mux.Handle(feedbackPath, feedbackHandler)

	var wrapped http.Handler = mux
	wrapped = withLogging(logger)(wrapped)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting api server", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}

	logger.Info("server stopped")
}

func withLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wr := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "error", rec, "path", r.URL.Path)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(wr, r)

			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wr.statusCode,
				"latency", time.Since(start).String(),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
