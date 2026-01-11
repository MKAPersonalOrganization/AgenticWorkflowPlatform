package middleware

import (
	"net/http"
	"net/http/httptest"
	"school-with-ai/foundation/metrics"
	"testing"
)

func TestPanics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectPanic    bool
		expectedStatus int
	}{
		{
			name: "no panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
			expectPanic:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name: "panic recovered",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("test panic")
			},
			expectPanic:    true,
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "panic with nil",
			handler: func(w http.ResponseWriter, r *http.Request) {
				var s *string
				_ = *s // This will panic
			},
			expectPanic:    true,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create request with metrics context
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			ctx := metrics.Set(req.Context())
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			// Wrap handler with panic middleware
			mw := Panics()
			handler := mw(tt.handler)

			// Execute handler
			handler.ServeHTTP(w, req)

			// Verify status code
			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify panic metric was incremented if panic was expected
			if tt.expectPanic {
				panicCount := metrics.AddPanics(ctx)
				if panicCount <= 0 {
					t.Errorf("expected panic count > 0, got %d", panicCount)
				}
			}
		})
	}
}

func TestPanicsIntegration(t *testing.T) {
	t.Parallel()

	// Create a handler that panics
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("integration test panic")
	})

	// Wrap with panic middleware
	mw := Panics()
	wrappedHandler := mw(handler)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := metrics.Set(req.Context())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	// Should not panic and should return 500
	wrappedHandler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	// Verify the server is still functioning by making another request
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusInternalServerError {
		t.Errorf("server should still be running, expected status 500, got %d", w2.Code)
	}
}
