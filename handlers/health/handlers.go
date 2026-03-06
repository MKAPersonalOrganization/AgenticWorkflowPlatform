package health

import (
	"context"
	"encoding/json"
	"net/http"
	"school-with-ai/foundation/logger"
	"sync"
	"time"
)

// Checker represents a dependency that can report its health status.
type Checker interface {
	// Check returns nil if the dependency is healthy, or an error describing
	// why it is not.
	Check(ctx context.Context) error
}

// CheckerFunc is an adapter to use ordinary functions as Checkers.
type CheckerFunc func(ctx context.Context) error

func (f CheckerFunc) Check(ctx context.Context) error { return f(ctx) }

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	log      *logger.Logger
	checkers map[string]Checker
}

// NewHealthHandler creates a new health handler.
// Pass dependency checkers keyed by name (e.g., "db", "redis").
func NewHealthHandler(log *logger.Logger, checkers ...func(*HealthHandler)) *HealthHandler {
	h := &HealthHandler{
		log:      log,
		checkers: make(map[string]Checker),
	}
	for _, opt := range checkers {
		opt(h)
	}
	return h
}

// WithChecker registers a named dependency checker.
func WithChecker(name string, c Checker) func(*HealthHandler) {
	return func(h *HealthHandler) {
		h.checkers[name] = c
	}
}

// Liveness returns simple status info if the service is alive.
// This endpoint is public — do not expose infrastructure details.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	info := map[string]any{
		"status": "up",
	}

	if err := json.NewEncoder(w).Encode(info); err != nil {
		h.log.Error(r.Context(), "liveness", "status", "encode error", "ERROR", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Readiness checks if the service is ready to accept traffic by verifying
// all registered dependency checkers. If any checker fails, the service is
// considered not ready and returns HTTP 503 Service Unavailable.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Use a short timeout for health checks — fail fast.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	type checkResult struct {
		Name   string
		Status string
		Err    string
	}

	results := make([]checkResult, 0, len(h.checkers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	allHealthy := true

	for name, checker := range h.checkers {
		wg.Add(1)
		go func(name string, checker Checker) {
			defer wg.Done()
			err := checker.Check(ctx)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				allHealthy = false
				results = append(results, checkResult{Name: name, Status: "unhealthy", Err: err.Error()})
				h.log.Error(ctx, "readiness check failed", "dependency", name, "error", err)
			} else {
				results = append(results, checkResult{Name: name, Status: "healthy"})
			}
		}(name, checker)
	}
	wg.Wait()

	status := "ready"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	// Build checks map for response
	checks := make(map[string]any, len(results))
	for _, r := range results {
		entry := map[string]string{"status": r.Status}
		if r.Err != "" {
			entry["error"] = r.Err
		}
		checks[r.Name] = entry
	}

	response := map[string]any{
		"status": status,
		"checks": checks,
	}

	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error(r.Context(), "readiness", "status", "encode error", "ERROR", err)
	}
}
