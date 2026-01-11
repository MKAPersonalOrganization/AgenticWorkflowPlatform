// Package middleware provides application middleware support.
package middleware

import (
	"net/http"
	"runtime/debug"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/metrics"
)

// Panics recovers from panics and converts the panic to an error so it is
// reported in Metrics and handled in Errors.
func Panics(log *logger.Logger) func(http.Handler) http.Handler {
	m := func(next http.Handler) http.Handler {
		h := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Defer a function to recover from a panic and set the err return
			// variable after the fact.
			defer func() {
				if rec := recover(); rec != nil {
					trace := debug.Stack()

					log.Error(ctx, "PANIC", "panic", rec, "trace", string(trace))

					metrics.AddPanics(ctx)

					// Return 500 Internal Server Error
					w.WriteHeader(http.StatusInternalServerError)
					if _, err := w.Write([]byte("Internal Server Error")); err != nil {
						log.Error(ctx, "failed to write panic response", "error", err)
					}
				}
			}()

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(h)
	}

	return m
}
