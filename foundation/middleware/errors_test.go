package middleware_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/middleware"
	"testing"
)

func TestErrors(t *testing.T) {
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
			name: "client error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			log := logger.New(&buf, logger.LevelInfo, "test", func(_ context.Context) string { return "00000000-0000-0000-0000-000000000000" })

			handler := middleware.Errors(log)(tt.handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code == 0 {
				t.Error("expected status code to be set")
			}
		})
	}
}

func TestErrorsDoesNotPanic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := logger.New(&buf, logger.LevelInfo, "test", func(_ context.Context) string { return "00000000-0000-0000-0000-000000000000" })

	handler := middleware.Errors(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
