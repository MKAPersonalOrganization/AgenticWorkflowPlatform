// Package middleware provides application middleware support.
package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"school-with-ai/foundation/errs"
	"school-with-ai/foundation/metrics"
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

					// Create an internal error that won't leak details to the client
					err := errs.Errorf(errs.InternalOnlyLog, "PANIC [%v] TRACE[%s]", rec, string(trace))

					metrics.AddPanics(ctx)

					// Store error in response writer for Error middleware to handle
					if rw, ok := w.(*responseWriter); ok {
						rw.err = err
						rw.statusCode = http.StatusInternalServerError
					} else {
						// Fallback if not wrapped
						w.WriteHeader(http.StatusInternalServerError)
						if _, err := w.Write([]byte("Internal Server Error")); err != nil {
							slog.Error("failed to write panic response", "error", err)
						}
					}
				}
			}()

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(h)
	}

	return m
}
