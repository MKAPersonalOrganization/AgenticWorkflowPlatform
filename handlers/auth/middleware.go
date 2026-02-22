package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/kratos"

	client "github.com/ory/client-go"
)

var (
	// ErrNoCookieHeader is returned when the Cookie header is missing from the request.
	ErrNoCookieHeader = errors.New("no cookie header")
	// ErrSessionNotActive is returned when the session is not active.
	ErrSessionNotActive = errors.New("session is not active")
)

// sessionCacheEntry holds a cached session with its expiration time.
type sessionCacheEntry struct {
	session   *client.Session
	expiresAt time.Time
}

const (
	sessionCacheTTL     = 30 * time.Second
	sessionCacheMaxSize = 10000
)

// Middleware wraps the Kratos client for authentication middleware.
type Middleware struct {
	kratosClient *kratos.KratosClient
	appURL       string
	cacheMu      sync.RWMutex
	sessionCache map[string]sessionCacheEntry
}

// NewMiddleware creates a new authentication middleware.
func NewMiddleware(kratosClient *kratos.KratosClient, appURL string) *Middleware {
	m := &Middleware{
		kratosClient: kratosClient,
		appURL:       appURL,
		sessionCache: make(map[string]sessionCacheEntry),
	}
	// Background goroutine to periodically evict expired entries.
	go m.evictExpired()
	return m
}

// evictExpired removes expired entries from the session cache every 60 seconds.
func (m *Middleware) evictExpired() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.cacheMu.Lock()
		for k, v := range m.sessionCache {
			if now.After(v.expiresAt) {
				delete(m.sessionCache, k)
			}
		}
		m.cacheMu.Unlock()
	}
}

// RequireAuth is middleware that requires a valid session
// If no session exists, it redirects to the login page
// If session requires 2FA (AAL2), it redirects to login with aal=aal2.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := m.validateSession(r)
		if err != nil {
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
			next.ServeHTTP(w, r)

			return
		}

		// Convert session to domain user
		user, err := domain.NewUserFromSession(session)
		if err != nil {
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

	// Hash the cookie to use as a cache key (avoid storing raw cookies in memory).
	h := sha256.Sum256([]byte(cookie))
	cacheKey := hex.EncodeToString(h[:])

	// Check cache first.
	now := time.Now()
	m.cacheMu.RLock()
	if entry, ok := m.sessionCache[cacheKey]; ok && now.Before(entry.expiresAt) {
		m.cacheMu.RUnlock()
		return entry.session, nil
	}
	m.cacheMu.RUnlock()

	// Validate session with Kratos
	session, err := m.kratosClient.ToSession(r.Context(), cookie)
	if err != nil {
		return nil, fmt.Errorf("session validation failed: %w", err)
	}

	// Check if session is active
	if session.Active == nil || !*session.Active {
		return nil, ErrSessionNotActive
	}

	// Store in cache with TTL. Evict half the entries if over capacity.
	m.cacheMu.Lock()
	if len(m.sessionCache) >= sessionCacheMaxSize {
		count := 0
		half := sessionCacheMaxSize / 2
		for k := range m.sessionCache {
			delete(m.sessionCache, k)
			count++
			if count >= half {
				break
			}
		}
	}
	m.sessionCache[cacheKey] = sessionCacheEntry{
		session:   session,
		expiresAt: now.Add(sessionCacheTTL),
	}
	m.cacheMu.Unlock()

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
