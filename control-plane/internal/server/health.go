package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Health struct {
	startTime time.Time
}

func NewHealth() *Health {
	return &Health{startTime: time.Now()}
}

func (h *Health) Check(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"service":   "harpia-api",
		"uptime":    time.Since(h.startTime).String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
