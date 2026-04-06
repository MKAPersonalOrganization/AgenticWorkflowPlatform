package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"aicompass/foundation/logger"
	"aicompass/foundation/health"
	"testing"
)

func newTestLogger() *logger.Logger {
	return logger.NewTextWithEvents(os.Stdout, logger.LevelInfo, "TEST", nil, logger.Events{})
}

func TestLiveness(t *testing.T) {
	t.Parallel()

	handler := health.NewHealthHandler(newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/liveness", nil)
	rec := httptest.NewRecorder()

	handler.Liveness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "up" {
		t.Errorf("status = %v, want %q", body["status"], "up")
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestReadiness_AllHealthy(t *testing.T) {
	t.Parallel()

	handler := health.NewHealthHandler(newTestLogger(),
		health.WithChecker("db", health.CheckerFunc(func(ctx context.Context) error {
			return nil
		})),
		health.WithChecker("redis", health.CheckerFunc(func(ctx context.Context) error {
			return nil
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ready" {
		t.Errorf("status = %v, want %q", body["status"], "ready")
	}

	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatal("checks not found in response")
	}

	for _, name := range []string{"db", "redis"} {
		check, ok := checks[name].(map[string]any)
		if !ok {
			t.Errorf("check %q not found", name)
			continue
		}
		if check["status"] != "healthy" {
			t.Errorf("check %q status = %v, want %q", name, check["status"], "healthy")
		}
	}
}

func TestReadiness_DependencyUnhealthy(t *testing.T) {
	t.Parallel()

	handler := health.NewHealthHandler(newTestLogger(),
		health.WithChecker("db", health.CheckerFunc(func(ctx context.Context) error {
			return errors.New("connection refused")
		})),
		health.WithChecker("redis", health.CheckerFunc(func(ctx context.Context) error {
			return nil
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "not_ready" {
		t.Errorf("status = %v, want %q", body["status"], "not_ready")
	}

	checks := body["checks"].(map[string]any)
	dbCheck := checks["db"].(map[string]any)
	if dbCheck["status"] != "unhealthy" {
		t.Errorf("db status = %v, want %q", dbCheck["status"], "unhealthy")
	}
	if dbCheck["error"] != "connection refused" {
		t.Errorf("db error = %v, want %q", dbCheck["error"], "connection refused")
	}
}

func TestReadiness_NoCheckers(t *testing.T) {
	t.Parallel()

	handler := health.NewHealthHandler(newTestLogger())

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "ready" {
		t.Errorf("status = %v, want %q", body["status"], "ready")
	}
}

func TestReadiness_AllUnhealthy(t *testing.T) {
	t.Parallel()

	handler := health.NewHealthHandler(newTestLogger(),
		health.WithChecker("db", health.CheckerFunc(func(ctx context.Context) error {
			return errors.New("db down")
		})),
		health.WithChecker("redis", health.CheckerFunc(func(ctx context.Context) error {
			return errors.New("redis down")
		})),
	)

	req := httptest.NewRequest(http.MethodGet, "/readiness", nil)
	rec := httptest.NewRecorder()

	handler.Readiness(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "not_ready" {
		t.Errorf("status = %v, want %q", body["status"], "not_ready")
	}
}
