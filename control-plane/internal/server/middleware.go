package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/harpia/control-plane/internal/cache"
)

func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logger.Info("request",
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency", latency.String(),
			"client_ip", c.ClientIP(),
			"request_id", c.GetString("request_id"),
		)
	}
}

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		logger.Error("panic recovered",
			"error", err,
			"path", c.Request.URL.Path,
			"request_id", c.GetString("request_id"),
		)
		c.AbortWithStatus(500)
	})
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = uuid.New().String()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func RateLimit(limiter *cache.RateLimiter, maxPerMinute int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = "anonymous"
			}

			allowed, err := limiter.Allow(r.Context(), tenantID, maxPerMinute)
			if err != nil {
				http.Error(w, "rate limit check failed", http.StatusInternalServerError)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
