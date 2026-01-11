package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"runtime"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	log *slog.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(log *slog.Logger) *HealthHandler {
	return &HealthHandler{
		log: log,
	}
}

// Liveness returns simple status info if the service is alive.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	info := map[string]any{
		"status":     "up",
		"host":       host,
		"GOMAXPROCS": runtime.GOMAXPROCS(0),
	}

	if err := json.NewEncoder(w).Encode(info); err != nil {
		h.log.Error("Failed to encode liveness response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Readiness checks if the service is ready to accept traffic.
// In a production system, this would check critical dependencies like
// database connectivity, message queues, etc.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// For now, we'll just return ready status
	// In a production system, you would check:
	// - Database connectivity
	// - KurrentDB connectivity
	// - Redis connectivity
	// - Any other critical dependencies
	response := map[string]any{
		"status": "ready",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode readiness response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
