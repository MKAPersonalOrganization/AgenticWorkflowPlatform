package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"school-with-ai/domain"

	client "github.com/ory/client-go"
)

var (
	// ErrNoCookieHeader is returned when the Cookie header is missing from the request.
	ErrNoCookieHeader = errors.New("no cookie header")
	// ErrSessionNotActive is returned when the session is not active.
	ErrSessionNotActive = errors.New("session is not active")
)

// Middleware wraps the Kratos client for authentication middleware.
type Middleware struct {
	kratosClient *KratosClient
	appURL       string
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(kratosClient *KratosClient) *Middleware {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		panic("required environment variable APP_URL is not set")
	}

	return &Middleware{
		kratosClient: kratosClient,
		appURL:       appURL,
	}
}

// RequireAuth is middleware that requires a valid session
// If no session exists, it redirects to the login page
// If session requires 2FA (AAL2), it redirects to login with aal=aal2.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := m.validateSession(r)
		if err != nil {
			slog.Debug("Session validation failed, redirecting to login",
				"path", r.URL.Path,
				"error", err)

			// Store the original URL for return_to
			returnTo := r.URL.String()
			loginURL := fmt.Sprintf("%s/auth/login?return_to=%s",
				m.appURL,
				url.QueryEscape(returnTo))

			http.Redirect(w, r, loginURL, http.StatusSeeOther)

			return
		}

		// Check if AAL2 (2FA) is required
		if session.AuthenticatorAssuranceLevel != nil {
			aal := string(*session.AuthenticatorAssuranceLevel)
			if session.Identity != nil && len(session.Identity.MetadataPublic) > 0 {
				// Check if higher AAL is required
				if requiredAAL, ok := session.Identity.MetadataPublic["required_aal"]; ok {
					if requiredAAL == "aal2" && aal != "aal2" {
						returnTo := r.URL.String()
						loginURL := fmt.Sprintf("%s/auth/login?aal=aal2&return_to=%s",
							m.appURL,
							url.QueryEscape(returnTo))

						http.Redirect(w, r, loginURL, http.StatusSeeOther)

						return
					}
				}
			}
		}

		// Convert session to domain user
		user, err := domain.NewUserFromSession(session)
		if err != nil {
			slog.Error("Failed to convert session to user", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)

			return
		}

		// Add user and session to context
		ctx := context.WithValue(r.Context(), domain.UserContextKey, user)
		ctx = context.WithValue(ctx, domain.SessionContextKey, session)

		// Call next handler with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that adds session to context if present
// but doesn't require authentication.
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := m.validateSession(r)
		if err != nil {
			// Session not valid, but we continue anyway
			slog.Debug("Optional auth: no valid session", "path", r.URL.Path)
			next.ServeHTTP(w, r)

			return
		}

		// Convert session to domain user
		user, err := domain.NewUserFromSession(session)
		if err != nil {
			slog.Warn("Failed to convert session to user", "error", err)
			next.ServeHTTP(w, r)

			return
		}

		// Add user and session to context
		ctx := context.WithValue(r.Context(), domain.UserContextKey, user)
		ctx = context.WithValue(ctx, domain.SessionContextKey, session)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validateSession checks if the request has a valid Kratos session.
func (m *Middleware) validateSession(r *http.Request) (*client.Session, error) {
	// Get session cookie from request
	cookie := r.Header.Get("Cookie")
	if cookie == "" {
		return nil, ErrNoCookieHeader
	}

	// Validate session with Kratos
	session, err := m.kratosClient.ToSession(r.Context(), cookie)
	if err != nil {
		return nil, fmt.Errorf("session validation failed: %w", err)
	}

	// Check if session is active
	if session.Active == nil || !*session.Active {
		return nil, ErrSessionNotActive
	}

	return session, nil
}

// RequireAuthFunc wraps an http.HandlerFunc with authentication.
func (m *Middleware) RequireAuthFunc(handler http.HandlerFunc) http.HandlerFunc {
	return m.RequireAuth(handler).ServeHTTP
}

// OptionalAuthFunc wraps an http.HandlerFunc with optional authentication.
func (m *Middleware) OptionalAuthFunc(handler http.HandlerFunc) http.HandlerFunc {
	return m.OptionalAuth(handler).ServeHTTP
}
