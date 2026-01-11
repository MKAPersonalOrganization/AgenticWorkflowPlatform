package metrics

import (
	"context"
	"expvar"
	"testing"
)

func TestMetrics(t *testing.T) {
	t.Parallel()

	// Reset metrics for testing
	m.requests = expvar.NewInt("test_requests")
	m.errors = expvar.NewInt("test_errors")
	m.panics = expvar.NewInt("test_panics")
	m.goroutines = expvar.NewInt("test_goroutines")

	ctx := Set(context.Background())

	t.Run("AddRequests", func(t *testing.T) {
		t.Parallel()

		initial := m.requests.Value()
		count := AddRequests(ctx)

		if count != initial+1 {
			t.Errorf("expected count %d, got %d", initial+1, count)
		}
	})

	t.Run("AddErrors", func(t *testing.T) {
		t.Parallel()

		initial := m.errors.Value()
		count := AddErrors(ctx)

		if count != initial+1 {
			t.Errorf("expected count %d, got %d", initial+1, count)
		}
	})

	t.Run("AddPanics", func(t *testing.T) {
		t.Parallel()

		initial := m.panics.Value()
		count := AddPanics(ctx)

		if count != initial+1 {
			t.Errorf("expected count %d, got %d", initial+1, count)
		}
	})

	t.Run("AddGoroutines", func(t *testing.T) {
		t.Parallel()

		count := AddGoroutines(ctx)

		if count <= 0 {
			t.Errorf("expected positive goroutine count, got %d", count)
		}
	})

	t.Run("WithoutContext", func(t *testing.T) {
		t.Parallel()

		// Should return 0 when metrics not in context
		ctx := context.Background()

		if count := AddRequests(ctx); count != 0 {
			t.Errorf("expected 0 without context, got %d", count)
		}

		if count := AddErrors(ctx); count != 0 {
			t.Errorf("expected 0 without context, got %d", count)
		}

		if count := AddPanics(ctx); count != 0 {
			t.Errorf("expected 0 without context, got %d", count)
		}

		if count := AddGoroutines(ctx); count != 0 {
			t.Errorf("expected 0 without context, got %d", count)
		}
	})
}
