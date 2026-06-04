package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
