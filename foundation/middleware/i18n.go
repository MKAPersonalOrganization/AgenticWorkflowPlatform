package middleware

import (
	"context"
	"net/http"
	"time"

	"school-with-ai/foundation/i18n"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type contextKey string

const (
	LanguageContextKey  contextKey = "language"
	DirectionContextKey contextKey = "direction"
	PrinterContextKey   contextKey = "printer"
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
			// e.g. if cookie says "fr" but we don't support it, we might want fallback?
			// i18n.Matcher will pick the closest match.
			t, _, _ := i18n.Matcher.Match(tag)
			tag = t
		}

		// Initialize Printer
		printer := message.NewPrinter(tag)
		dir := i18n.GetDirection(tag)

		ctx := context.WithValue(r.Context(), LanguageContextKey, tag.String())
		ctx = context.WithValue(ctx, DirectionContextKey, dir)
		ctx = context.WithValue(ctx, PrinterContextKey, printer)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetLanguage returns the language tag from context, or "en" if missing.
func GetLanguage(ctx context.Context) string {
	if val, ok := ctx.Value(LanguageContextKey).(string); ok {
		return val
	}
	return "en"
}

// GetDirection returns "rtl" or "ltr" from context.
func GetDirection(ctx context.Context) string {
	if val, ok := ctx.Value(DirectionContextKey).(string); ok {
		return val
	}
	return "ltr"
}

// GetPrinter returns the message printer from context.
func GetPrinter(ctx context.Context) *message.Printer {
	if val, ok := ctx.Value(PrinterContextKey).(*message.Printer); ok {
		return val
	}
	return message.NewPrinter(language.English)
}
