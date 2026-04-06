package middleware_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"aicompass/foundation/logger"
	"aicompass/foundation/middleware"
	"testing"
)

func TestLogging(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedStatus int
		method         string
		path           string
	}{
		{
			name: "successful GET request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectedStatus: http.StatusOK,
			method:         http.MethodGet,
			path:           "/test",
		},
		{
			name: "POST request with error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedStatus: http.StatusBadRequest,
			method:         http.MethodPost,
			path:           "/api/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			log := logger.New(&buf, logger.LevelInfo, "test", func(_ context.Context) string { return "00000000-0000-0000-0000-000000000000" })
			handler := middleware.Logger(log)(tt.handler)
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			logOutput := buf.String()
			if logOutput == "" {
				t.Error("expected log output, got empty string")
			}
		})
	}
}
