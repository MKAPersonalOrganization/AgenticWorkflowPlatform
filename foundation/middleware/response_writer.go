package middleware

import "net/http"

// responseWriter wraps http.ResponseWriter to capture status codes.
// This is shared across middleware to track response status.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	if rw.statusCode == 0 || rw.statusCode == http.StatusOK {
		rw.statusCode = code
	}
	rw.ResponseWriter.WriteHeader(code)
}
