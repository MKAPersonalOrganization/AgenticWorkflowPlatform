// Package middleware provides application middleware support.
package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"school-with-ai/foundation/errs"
	"school-with-ai/foundation/logger"
)

// Errors handles errors coming out of the call chain.
func Errors(log *logger.Logger) func(http.Handler) http.Handler {
	m := func(next http.Handler) http.Handler {
		h := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Wrap response writer to capture errors
			rw := w
			if _, ok := w.(*responseWriter); !ok {
				rw = &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			}

			next.ServeHTTP(rw, r)

			// Check if an error was captured
			if wr, ok := rw.(*responseWriter); ok && wr.err != nil {
				err := wr.err

				var appErr *errs.Error
				if !errors.As(err, &appErr) {
					appErr = errs.Errorf(errs.Internal, "Internal Server Error")
				}

				log.Error(ctx, "handled error during request",
					"err", err,
					"source_err_file", path.Base(appErr.FileName),
					"source_err_func", path.Base(appErr.FuncName))

				// Don't leak internal-only errors to the client
				if appErr.Code == errs.InternalOnlyLog {
					appErr = errs.Errorf(errs.Internal, "Internal Server Error")
				}

				// Write the error response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(appErr.HTTPStatus())

				data, err := json.Marshal(appErr)
				if err != nil {
					log.Error(ctx, "failed to marshal error response", "error", err)
					return
				}

				if _, err := w.Write(data); err != nil {
					log.Error(ctx, "failed to write error response", "error", err)
				}
			}
		}

		return http.HandlerFunc(h)
	}

	return m
}

// responseWriter wraps http.ResponseWriter to capture status codes and errors.
// This is shared across middleware to maintain state.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	err        error
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if rw.statusCode == 0 || rw.statusCode == http.StatusOK {
		rw.statusCode = code
	}
	rw.ResponseWriter.WriteHeader(code)
}
