package middleware

import (
	"aicompass/foundation/i18n"
	"net/http"
	"time"

	"golang.org/x/text/language"
)

// I18n Middleware detects the user's language and injects it into the context.
// Priority:
// 1. Query parameter "lang"
// 2. Cookie "lang"
// 3. Accept-Language header
// 4. Fallback (English)
func I18n(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tag language.Tag
		found := false

		// 1. Check Query Param
		if langParam := r.URL.Query().Get("lang"); langParam != "" {
			t, err := language.Parse(langParam)
			if err == nil {
				tag = t
				found = true
				// Set cookie to persist selection
				http.SetCookie(w, &http.Cookie{
					Name:     "lang",
					Value:    t.String(),
					Path:     "/",
					Expires:  time.Now().Add(365 * 24 * time.Hour),
					HttpOnly: false, // Allow JS access if needed
				})
			}
		}

		// 2. Check Cookie
		if !found {
			if cookie, err := r.Cookie("lang"); err == nil {
				t, err := language.Parse(cookie.Value)
				if err == nil {
					tag = t
					found = true
				}
			}
		}

		// 3. Check Accept-Language Header & Match against supported languages
		// Note: i18n.Matcher uses English as fallback, so this always returns a valid tag
		accept := r.Header.Get("Accept-Language")
		if !found {
			t, _, _ := i18n.Matcher.Match(language.Make(accept))
			tag = t
		} else {
			// Even if found via cookie/param, we should match it against supported to ensure validity
			t, _, _ := i18n.Matcher.Match(tag)
			tag = t
		}

		// Use helper to inject into context
		ctx := i18n.WithLanguage(r.Context(), tag)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

