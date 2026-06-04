package server

import (
	"encoding/json"
	"net/http"
	"time"
)

type Health struct {
	startTime time.Time
}

func NewHealth() *Health {
	return &Health{startTime: time.Now()}
}

func (h *Health) Check(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "ok",
		"service":   "harpia-api",
		"uptime":    time.Since(h.startTime).String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
