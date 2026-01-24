// Package middleware provides application middleware support.
package middleware

import (
	"net/http"

	"school-with-ai/foundation/metrics"
)

// Metrics updates program counters for requests, errors, and goroutines.
func Metrics() func(http.Handler) http.Handler {
	m := func(next http.Handler) http.Handler {
		h := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = metrics.Set(ctx)

			// Wrap response writer to capture status and errors
			rw := w
			if _, ok := w.(*responseWriter); !ok {
				rw = &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			}

			next.ServeHTTP(rw, r.WithContext(ctx))

			n := metrics.AddRequests(ctx)

			if n%1000 == 0 {
				metrics.AddGoroutines(ctx)
			}

			// Track errors based on status code
			if wr, ok := rw.(*responseWriter); ok {
				if wr.statusCode < 200 || wr.statusCode >= 300 {
					metrics.AddErrors(ctx)
				}
			}
		}

		return http.HandlerFunc(h)
	}

	return m
}
