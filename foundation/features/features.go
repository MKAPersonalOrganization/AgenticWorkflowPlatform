// Package features carries experimental feature flags through the request
// context so page handlers and templ components can gate UI without threading
// props through every call site (same pattern as foundation/i18n).
//
// Flags are resolved once at startup from configuration and stamped onto
// every request by Middleware.
package features

import (
	"context"
	"net/http"
)

// contextKey is a private type for context keys.
type contextKey string

const (
	knowledgeBaseKey contextKey = "knowledge-base"
	demoKey          contextKey = "demo"
)

// KnowledgeBase reports whether the experimental knowledge-base
// (Edgequake / RAG) feature is enabled for this request. Defaults to false
// when the flag was never stamped on the context.
func KnowledgeBase(ctx context.Context) bool {
	enabled, _ := ctx.Value(knowledgeBaseKey).(bool)
	return enabled
}

// WithKnowledgeBase returns a new context with the knowledge-base flag set.
func WithKnowledgeBase(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, knowledgeBaseKey, enabled)
}

// Demo reports whether the app is running in demo mode (ephemeral in-process
// infrastructure, no real users). UI uses this to render outward-facing
// actions like payments as visibly disabled.
func Demo(ctx context.Context) bool {
	demo, _ := ctx.Value(demoKey).(bool)
	return demo
}

// WithDemo returns a new context with the demo-mode flag set.
func WithDemo(ctx context.Context, demo bool) context.Context {
	return context.WithValue(ctx, demoKey, demo)
}

// Middleware stamps the feature flags onto every request context.
func Middleware(knowledgeBase, demo bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithKnowledgeBase(r.Context(), knowledgeBase)
			ctx = WithDemo(ctx, demo)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
