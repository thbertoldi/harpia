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

	"github.com/harpia/control-plane/gen/harpia/agents/v1/agentsv1connect"
	"github.com/harpia/control-plane/gen/harpia/feedback/v1/feedbackv1connect"
	"github.com/harpia/control-plane/gen/harpia/identity/v1/identityv1connect"
	"github.com/harpia/control-plane/gen/harpia/tasks/v1/tasksv1connect"
	"github.com/harpia/control-plane/internal/agents"
	"github.com/harpia/control-plane/internal/config"
	"github.com/harpia/control-plane/internal/database"
	"github.com/harpia/control-plane/internal/feedback"
	"github.com/harpia/control-plane/internal/identity"
	"github.com/harpia/control-plane/internal/tasks"
	"github.com/harpia/control-plane/internal/workflow"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx := context.Background()

	var (
		taskRepo  *tasks.Repository
		agentRepo *agents.Repository
	)

	if cfg.DatabaseURL != "" {
		pool, err := database.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database connection failed, running without persistence", "error", err)
		} else {
			taskRepo = tasks.NewRepository(pool)
			agentRepo = agents.NewRepository(pool)
			defer pool.Close()
		}
	}

	var temporalClient *workflow.TemporalClient
	if cfg.TemporalHost != "" {
		var err error
		temporalClient, err = workflow.NewTemporalClient(cfg.TemporalHost)
		if err != nil {
			logger.Error("temporal client failed, running without workflow engine", "error", err)
		} else {
			defer temporalClient.Close()
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"service":   "harpia-api",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	taskHandler := tasks.NewTaskHandler(taskRepo, temporalClient)
	agentsPath, agentsHandler := agentsv1connect.NewAgentServiceHandler(agents.NewAgentHandler(agentRepo))
	tasksPath, tasksHandler := tasksv1connect.NewTaskServiceHandler(taskHandler)
	identityPath, identityHandler := identityv1connect.NewIdentityServiceHandler(identity.NewIdentityHandler())
	feedbackPath, feedbackHandler := feedbackv1connect.NewFeedbackServiceHandler(feedback.NewFeedbackHandler())

	mux.Handle(agentsPath, agentsHandler)
	mux.Handle(tasksPath, tasksHandler)
	mux.Handle(identityPath, identityHandler)
	mux.Handle(feedbackPath, feedbackHandler)

	wrapped := withLogging(logger)(mux)

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
