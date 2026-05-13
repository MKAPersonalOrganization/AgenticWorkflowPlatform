package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// sessionCacheEntry holds a cached user with its expiration time.
type sessionCacheEntry struct {
	user      *User
	expiresAt time.Time
}

const (
	sessionCacheTTL     = 30 * time.Second
	sessionCacheMaxSize = 10000
)

// Middleware provides authentication middleware using session-based identity.
type Middleware struct {
	users        UserResolver
	sessions     SessionReader
	appURL       string
	cacheMu      sync.RWMutex
	userCache    map[string]sessionCacheEntry
	stopEviction context.CancelFunc
}

// NewMiddleware creates a new authentication middleware.
// users resolves user records; sessions reads the session cookie.
func NewMiddleware(users UserResolver, sessions SessionReader, appURL string, ctx context.Context) *Middleware {
	evictCtx, cancel := context.WithCancel(ctx)
	m := &Middleware{
		users:        users,
		sessions:     sessions,
		appURL:       appURL,
		userCache:    make(map[string]sessionCacheEntry),
		stopEviction: cancel,
	}
	go m.evictExpired(evictCtx)
	return m
}

// Shutdown stops the cache eviction goroutine.
func (m *Middleware) Shutdown() {
	if m.stopEviction != nil {
		m.stopEviction()
	}
}

// evictExpired removes expired entries from the user cache every 60 seconds.
func (m *Middleware) evictExpired(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			m.cacheMu.Lock()
			for k, v := range m.userCache {
				if now.After(v.expiresAt) {
					delete(m.userCache, k)
				}
			}
			m.cacheMu.Unlock()
		}
	}
}

// RequireAuth is middleware that requires a valid session.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := m.resolveUser(r)
		if err != nil || user == nil {
			returnTo := r.URL.String()
			loginURL := fmt.Sprintf("%s/auth/login?return_to=%s",
				m.appURL,
				url.QueryEscape(returnTo))
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that adds user to context if present.
func (m *Middleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := m.resolveUser(r)
		if err != nil || user == nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// resolveUser checks the session cookie and resolves the user (with caching).
func (m *Middleware) resolveUser(r *http.Request) (*User, error) {
	userID, ok := m.sessions.GetUserID(r)
	if !ok {
		return nil, nil
	}

	// Build cache key from userID
	h := sha256.Sum256([]byte(userID))
	cacheKey := hex.EncodeToString(h[:])

	now := time.Now()
	m.cacheMu.RLock()
	if entry, ok := m.userCache[cacheKey]; ok && now.Before(entry.expiresAt) {
		m.cacheMu.RUnlock()
		return entry.user, nil
	}
	m.cacheMu.RUnlock()

	// Cache miss — load from database
	user, err := m.users.GetUserByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	m.cacheMu.Lock()
	if len(m.userCache) >= sessionCacheMaxSize {
		count := 0
		half := sessionCacheMaxSize / 2
		for k := range m.userCache {
			delete(m.userCache, k)
			count++
			if count >= half {
				break
			}
		}
	}
	m.userCache[cacheKey] = sessionCacheEntry{
		user:      user,
		expiresAt: now.Add(sessionCacheTTL),
	}
	m.cacheMu.Unlock()

	return user, nil
}

// RequireAuthFunc wraps an http.HandlerFunc with authentication.
func (m *Middleware) RequireAuthFunc(handler http.HandlerFunc) http.HandlerFunc {
	return m.RequireAuth(handler).ServeHTTP
}

// OptionalAuthFunc wraps an http.HandlerFunc with optional authentication.
func (m *Middleware) OptionalAuthFunc(handler http.HandlerFunc) http.HandlerFunc {
	return m.OptionalAuth(handler).ServeHTTP
}
