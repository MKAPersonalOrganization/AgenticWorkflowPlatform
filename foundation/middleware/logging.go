// Package middleware provides application middleware support.
package middleware

import (
	"fmt"
	"net/http"
	"time"

	"school-with-ai/foundation/logger"
)

// Logger writes information about the request to the logs.
func Logger(log *logger.Logger) func(http.Handler) http.Handler {
	m := func(next http.Handler) http.Handler {
		h := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			now := time.Now()

			path := r.URL.Path
			if r.URL.RawQuery != "" {
				path = fmt.Sprintf("%s?%s", path, r.URL.RawQuery)
			}

			log.Info(ctx, "request started", "method", r.Method, "path", path, "remoteaddr", r.RemoteAddr)

			// Wrap response writer to capture status and errors
			rw := w
			if _, ok := w.(*responseWriter); !ok {
				rw = &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			}

			next.ServeHTTP(rw, r)

			// Get the status code
			statusCode := http.StatusOK
			if wr, ok := rw.(*responseWriter); ok {
				statusCode = wr.statusCode
			}

			log.Info(ctx, "request completed", "method", r.Method, "path", path, "remoteaddr", r.RemoteAddr,
				"statuscode", statusCode, "since", time.Since(now).String())
		}

		return http.HandlerFunc(h)
	}

	return m
}
