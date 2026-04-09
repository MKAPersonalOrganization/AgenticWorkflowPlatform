// Package middleware provides application middleware support.
package middleware

import (
	"aicompass/foundation/errs"
	"aicompass/foundation/metrics"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Panics recovers from panics and converts the panic to an error so it is
// reported in Metrics and handled in Errors.
func Panics() func(http.Handler) http.Handler {
	m := func(next http.Handler) http.Handler {
		h := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Defer a function to recover from a panic and set the err return
			// variable after the fact.
			defer func() {
				if rec := recover(); rec != nil {
					trace := debug.Stack()

					// Log the panic with full details (never exposed to client)
					err := errs.Errorf(errs.InternalOnlyLog, "PANIC [%v] TRACE[%s]", rec, string(trace))
					slog.Error("panic recovered", "error", err)

					metrics.AddPanics(ctx)

					// Return generic error to client (don't leak panic details)
					w.WriteHeader(http.StatusInternalServerError)
					if _, writeErr := w.Write([]byte("Internal Server Error")); writeErr != nil {
						slog.Error("failed to write panic response", "error", writeErr)
					}
				}
			}()

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(h)
	}

	return m
}
