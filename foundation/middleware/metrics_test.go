package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"school-with-ai/foundation/middleware"
	"testing"
)

func TestMetrics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedStatus int
	}{
		{
			name: "successful request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "error request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := middleware.Metrics()(tt.handler)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestMetrics_ResponseController(t *testing.T) {
	t.Parallel()
	// Test that http.ResponseController can work with our wrapped response writer.
	// This is critical for SSE streaming to disable write deadlines in production.
	//
	// Note: httptest.ResponseRecorder doesn't support SetWriteDeadline, so this test
	// verifies that our wrapper implements Unwrap() correctly. In production with a
	// real http.Server, the underlying connection will support deadlines.
	handler := middleware.Metrics()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that our responseWriter implements Unwrap()
		type unwrapper interface {
			Unwrap() http.ResponseWriter
		}
		if _, ok := w.(unwrapper); !ok {
			t.Error("responseWriter doesn't implement Unwrap()")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}
