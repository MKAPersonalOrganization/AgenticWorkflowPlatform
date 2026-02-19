// Package middleware provides application middleware support.
package middleware

import "net/http"

// SecurityHeaders sets HTTP response headers that protect against common
// web vulnerabilities: clickjacking, MIME-type sniffing, XSS, and more.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Prevent the page from being embedded in iframes (clickjacking).
			h.Set("X-Frame-Options", "DENY")

			// Stop browsers from MIME-sniffing the Content-Type.
			h.Set("X-Content-Type-Options", "nosniff")

			// Control the Referer header sent with outgoing requests.
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Disable browser features the app does not use.
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

			// Content Security Policy — allow self-hosted assets, inline
			// styles required by templ/HTMX, and the jsdelivr CDN used for
			// markdown-it, KaTeX, Alpine.js, and xstate.
			// Alpine.js requires 'unsafe-eval' because it uses new Function()
			// to evaluate x-data, x-show, @click and other expressions.
			h.Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com; "+
					"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://fonts.googleapis.com; "+
					"img-src 'self' data: blob:; "+
					"connect-src 'self'; "+
					"font-src 'self' https://cdn.jsdelivr.net https://fonts.gstatic.com; "+
					"frame-ancestors 'none'")

			// Enforce HTTPS via HSTS when served over TLS.
			// max-age=63072000 = 2 years.
			if r.TLS != nil {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
