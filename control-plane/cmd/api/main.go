package main

import (
	"context"
	"crypto/rand"
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
	"github.com/harpia/control-plane/gen/harpia/budget/v1/budgetv1connect"
	"github.com/harpia/control-plane/gen/harpia/executors/v1/executorsv1connect"
	"github.com/harpia/control-plane/gen/harpia/feedback/v1/feedbackv1connect"
	"github.com/harpia/control-plane/gen/harpia/identity/v1/identityv1connect"
	"github.com/harpia/control-plane/gen/harpia/llm_config/v1/llm_configv1connect"
	"github.com/harpia/control-plane/gen/harpia/plans/v1/plansv1connect"
	"github.com/harpia/control-plane/gen/harpia/tasks/v1/tasksv1connect"
	"github.com/harpia/control-plane/internal/agents"
	"github.com/harpia/control-plane/internal/artifacts"
	"github.com/harpia/control-plane/internal/budget"
	"github.com/harpia/control-plane/internal/cache"
	"github.com/harpia/control-plane/internal/chat"
	"github.com/harpia/control-plane/internal/config"
	"github.com/harpia/control-plane/internal/database"
	"github.com/harpia/control-plane/internal/executors"
	"github.com/harpia/control-plane/internal/executors/bootstrap"
	"github.com/harpia/control-plane/internal/feedback"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/llm_config"
	cryptoenv "github.com/harpia/control-plane/internal/llm_config/crypto"
	"github.com/harpia/control-plane/internal/planassistant"
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
	chatStore := chat.NewPostgresStore(pool)
	artifactRepo := artifacts.NewRepository(pool)

	tenantObjectStore := storage.NewTenantObjectStore(cfg.GarageBucket)
	garageStore, err := artifacts.NewGarageStore(artifacts.GarageConfig{
		Endpoint:  cfg.GarageURL,
		Bucket:    cfg.GarageBucket,
		Region:    cfg.GarageRegion,
		AccessKey: cfg.GarageAccessKey,
		SecretKey: cfg.GarageSecretKey,
	}, tenantObjectStore)
	if err != nil {
		fatal("create garage store failed", "error", err)
	}

	executorRuntime := bootstrap.NewRuntime(bootstrap.Dependencies{
		ArtifactRepo: artifactRepo,
		PayloadStore: garageStore,
	})

	planActivities := &workflow.PlanActivities{
		Runtime:      plans.NewRuntimeRepository(planRepo, executorRepo, chatStore),
		Integrations: executorRuntime.Integrations,
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

const (
	temporalClientStartupTimeout = 60 * time.Second
	temporalClientRetryInterval  = time.Second
)

func setupTemporalClient(ctx context.Context, cfg *config.Config, logger *slog.Logger) *workflow.TemporalClient {
	if cfg.TemporalHost == "" {
		return nil
	}

	deadlineCtx, cancel := context.WithTimeout(ctx, temporalClientStartupTimeout)
	defer cancel()

	var lastErr error
	for attempt := 1; ; attempt++ {
		temporalClient, err := workflow.NewTemporalClient(cfg.TemporalHost)
		if err == nil {
			if attempt > 1 {
				logger.Info("temporal client connected", "host", cfg.TemporalHost, "attempt", attempt)
			}
			return temporalClient
		}
		lastErr = err

		if deadlineCtx.Err() != nil {
			logger.Warn("temporal client failed, running without workflow engine", "host", cfg.TemporalHost, "error", lastErr)
			return nil
		}

		logger.Warn("temporal client dial failed, retrying", "host", cfg.TemporalHost, "attempt", attempt, "error", err)
		timer := time.NewTimer(temporalClientRetryInterval)
		select {
		case <-deadlineCtx.Done():
			timer.Stop()
			logger.Warn("temporal client failed, running without workflow engine", "host", cfg.TemporalHost, "error", lastErr)
			return nil
		case <-timer.C:
		}
	}
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
	if err := agents.EnsureAgentTypesForTenant(ctx, pool, tenantID); err != nil {
		fatal("seed agent types failed", "error", err)
	}
	if err := executors.EnsureTenantAgentInstallations(ctx, pool, tenantID); err != nil {
		fatal("seed agent installations failed", "error", err)
	}
	logger.Info("executor catalog, entitlements, and agent bootstrap ensured")

	taskRepo := tasks.NewRepository(pool)
	planRepo := plans.NewRepository(pool)
	agentRepo := agents.NewRepository(pool)
	executorRepo := executors.NewRepository(pool)
	chatStore := chat.NewPostgresStore(pool)

	cacheResources := setupAPICache(ctx, cfg.ValkeyURL, logger)
	if cacheResources.client != nil {
		defer cacheResources.client.Close()
	}
	temporalClient := setupTemporalClient(ctx, cfg, logger)
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
	assistantController := &planassistant.Controller{
		Chat:      chatStore,
		Catalog:   &plans.AssistantCatalog{Executors: executorRepo},
		Configs:   &plans.AssistantConfigurationStore{Repo: planRepo, Executors: executorRepo},
		Templates: &plans.AssistantTemplates{Repo: planRepo},
	}
	var planHandler *plans.PlanHandler
	if temporalClient != nil {
		planHandler, err = plans.NewPlanHandler(planRepo, executorRepo, scheduleManager, chatStore, assistantController, temporalClient)
	} else {
		planHandler, err = plans.NewPlanHandler(planRepo, executorRepo, scheduleManager, chatStore, assistantController)
	}
	if err != nil {
		fatal("create plan handler failed", "error", err)
	}

	executorHandler, err := executors.NewHandler(executorRepo, executors.DefaultConfigValidators())
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
	llmRepo := llm_config.NewRepository(pool)
	llmKeyring, err := loadLLMKeyring(cfg, logger)
	if err != nil {
		fatal("load llm keyring failed", "error", err)
	}
	llmResolver := llm_config.NewResolver(
		llmRepo,
		llmKeyring,
		llm_config.NewEnvPlatformKeyStore(),
		llm_config.BlockedProvidersFromEnv(),
	)
	llmHandler, err := llm_config.NewHandler(llm_config.HandlerOptions{
		Repo:     llmRepo,
		Keyring:  llmKeyring,
		Resolver: llmResolver,
	})
	if err != nil {
		fatal("create llm config handler failed", "error", err)
	}
	budgetHandler, err := budget.NewHandler(budget.HandlerOptions{
		Repo: budget.NewRepository(pool),
	})
	if err != nil {
		fatal("create budget handler failed", "error", err)
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
	llmConfigPath, llmConfigHandler := llm_configv1connect.NewLLMConfigServiceHandler(llm_config.NewPublicHandler(llmHandler), requestContext)
	budgetPath, budgetConnectHandler := budgetv1connect.NewBudgetPolicyServiceHandler(budgetHandler, requestContext)
	internalLLMPath, internalLLMHandler := llm_config.NewInternalResolveHandler(
		llmHandler,
		connect.WithInterceptors(identity.NewInternalServiceInterceptor(identity.InternalServiceOptions{
			Token:        cfg.InternalAuthToken,
			AllowDevAuth: cfg.AllowDevAuth,
		})),
	)
	internalBudgetPath, internalBudgetConnectHandler := budgetv1connect.NewBudgetPolicyServiceHandler(
		budgetHandler,
		connect.WithInterceptors(identity.NewInternalServiceInterceptor(identity.InternalServiceOptions{
			Token:        cfg.InternalAuthToken,
			AllowDevAuth: cfg.AllowDevAuth,
		})),
	)
	// Internal ArtifactService for the Python agent worker (RunAgentActivity):
	// loads upstream artifacts and persists agent output. Authenticated with the
	// internal-service token (not the public request-context interceptor); tenant
	// comes from the X-Tenant-ID header.
	internalArtifactsPath, internalArtifactsConnectHandler := artifactsv1connect.NewArtifactServiceHandler(
		artifactHandler,
		connect.WithInterceptors(identity.NewInternalServiceInterceptor(identity.InternalServiceOptions{
			Token:        cfg.InternalAuthToken,
			AllowDevAuth: cfg.AllowDevAuth,
		})),
	)

	mux.Handle(agentsPath, agentsHandler)
	mux.Handle(executorsPath, executorsHandler)
	mux.Handle(tasksPath, tasksHandler)
	mux.Handle(plansPath, plansHandler)
	mux.Handle(artifactsPath, artifactsHandler)
	mux.Handle(identityPath, identityHandler)
	mux.Handle(feedbackPath, feedbackHandler)
	mux.Handle(llmConfigPath, llmConfigHandler)
	mux.Handle(budgetPath, budgetConnectHandler)
	mux.Handle(internalLLMPath, internalLLMHandler)
	mux.Handle("/internal"+internalBudgetPath, http.StripPrefix("/internal", internalBudgetConnectHandler))
	mux.Handle("/internal"+internalArtifactsPath, http.StripPrefix("/internal", internalArtifactsConnectHandler))

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

func loadLLMKeyring(cfg *config.Config, logger *slog.Logger) (*cryptoenv.Keyring, error) {
	keyringCfg := cryptoenv.DefaultKeyringConfig()
	keyringCfg.MountDir = os.Getenv("HARPIA_LLM_KEK_MOUNT_DIR")

	keyring, err := cryptoenv.LoadKeyring(keyringCfg)
	if err == nil {
		return keyring, nil
	}
	if !cfg.AllowDevAuth {
		return nil, err
	}

	devKey := make([]byte, 32)
	if _, readErr := rand.Read(devKey); readErr != nil {
		return nil, fmt.Errorf("generate dev llm kek: %w", readErr)
	}
	keyring, keyringErr := cryptoenv.NewKeyring(
		[]cryptoenv.KEKMaterial{{Version: "dev", Key: devKey}},
		"dev",
	)
	if keyringErr != nil {
		return nil, fmt.Errorf("create dev llm keyring: %w", keyringErr)
	}
	logger.Warn(
		"llm kek not configured; using ephemeral dev keyring",
		"error", err.Error(),
	)
	return keyring, nil
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

func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
