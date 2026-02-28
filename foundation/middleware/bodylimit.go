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
//
// Optional excludePaths may be provided — requests whose path has any of these
// prefixes skip the limit entirely (they are expected to apply their own
// limit via a route-group middleware).
func BodyLimit(limit int64, excludePaths ...string) func(http.Handler) http.Handler {
	if limit <= 0 {
		limit = defaultBodyLimit
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip multipart/form-data requests — file upload handlers
			// control their own size limits via ParseMultipartForm.
			ct := r.Header.Get("Content-Type")
			isMultipart := strings.HasPrefix(ct, "multipart/form-data")

			// Skip excluded paths — they set their own limits.
			excluded := false
			for _, prefix := range excludePaths {
				if strings.HasPrefix(r.URL.Path, prefix) {
					excluded = true
					break
				}
			}

			// Only limit bodies for non-excluded, non-multipart requests that have a body.
			if !excluded && !isMultipart && r.Body != nil && r.ContentLength != 0 {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}

			next.ServeHTTP(w, r)
		})
	}
}
