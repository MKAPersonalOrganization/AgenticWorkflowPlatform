// Package middleware provides application middleware support.
package middleware

import (
	"net/http"
	"strings"
)

// defaultBodyLimit is 1 MB — enough for typical JSON API requests.
const defaultBodyLimit int64 = 1 << 20 // 1 MB

// BodyLimit restricts the maximum size of incoming request bodies.
// When the limit is exceeded, the server returns 413 Request Entity Too Large.
//
// Pass 0 to use the default limit (1 MB).
// This middleware should be applied globally. Multipart/form-data requests
// (file uploads) are excluded so that handlers can control their own limits
// via ParseMultipartForm.
func BodyLimit(limit int64) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = defaultBodyLimit
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip multipart/form-data requests — file upload handlers
			// control their own size limits via ParseMultipartForm.
			ct := r.Header.Get("Content-Type")
			isMultipart := strings.HasPrefix(ct, "multipart/form-data")

			// Only limit bodies for non-multipart requests that have a body.
			if !isMultipart && r.Body != nil && r.ContentLength != 0 {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}

			next.ServeHTTP(w, r)
		})
	}
}
