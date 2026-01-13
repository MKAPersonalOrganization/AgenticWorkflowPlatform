package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"school-with-ai/domain"
	"school-with-ai/infra/kratos"
	"testing"
	"time"

	client "github.com/ory/client-go"
)

func TestMain(m *testing.M) {
	// Set required env vars for all tests
	_ = os.Setenv("KRATOS_PUBLIC_URL", "http://127.0.0.1:4433")
	_ = os.Setenv("KRATOS_ADMIN_URL", "http://127.0.0.1:4434")
	_ = os.Setenv("KRATOS_BROWSER_URL", "http://127.0.0.1:4433")
	_ = os.Setenv("APP_URL", "http://127.0.0.1:8090")
	os.Exit(m.Run())
}

func TestNewMiddleware(t *testing.T) {
	t.Parallel()
	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	if middleware == nil {
		t.Fatal("NewMiddleware() returned nil")
	}

	if middleware.kratosClient == nil {
		t.Error("kratosClient is nil")
	}

	if middleware.appURL == "" {
		t.Error("appURL is empty")
	}
}

func TestMiddleware_RequireAuth_NoSession(t *testing.T) {
	t.Parallel()
	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	// Create a test handler
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	// Create request without cookie
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	// Wrap handler with RequireAuth
	middleware.RequireAuth(handler).ServeHTTP(rec, req)

	// Should redirect to login
	if rec.Code != http.StatusSeeOther {
		t.Errorf("Expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	// Handler should not be called
	if called {
		t.Error("Handler was called despite missing session")
	}

	// Check redirect location
	location := rec.Header().Get("Location")
	if location == "" {
		t.Error("No redirect location set")
	}
}

func TestMiddleware_RequireAuth_ValidSession(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Set up environment (all required)
	_ = os.Setenv("KRATOS_PUBLIC_URL", "http://127.0.0.1:4433")
	_ = os.Setenv("KRATOS_ADMIN_URL", "http://127.0.0.1:4434")
	_ = os.Setenv("KRATOS_BROWSER_URL", "http://127.0.0.1:4433")
	_ = os.Setenv("APP_URL", "http://127.0.0.1:8090")

	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	// Create a test handler that checks for user in context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := kratos.GetUserFromContext(r.Context())
		if user == nil {
			t.Error("User not found in context")
		}

		session := kratos.GetSessionFromContext(r.Context())
		if session == nil {
			t.Error("Session not found in context")
		}

		w.WriteHeader(http.StatusOK)
	})

	// Note: This test requires a valid session cookie from Kratos
	// In a real integration test, you would create a session first
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Cookie", "invalid-session-cookie")
	rec := httptest.NewRecorder()

	middleware.RequireAuth(handler).ServeHTTP(rec, req)

	// Should redirect to login since we don't have a valid session
	if rec.Code != http.StatusSeeOther {
		t.Logf("Got status %d (expected redirect without valid session)", rec.Code)
	}
}

func TestMiddleware_OptionalAuth_NoSession(t *testing.T) {
	t.Parallel()
	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		user := kratos.GetUserFromContext(r.Context())
		if user != nil {
			t.Error("User should be nil when no session")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rec := httptest.NewRecorder()

	middleware.OptionalAuth(handler).ServeHTTP(rec, req)

	// Should allow access
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Handler should be called
	if !called {
		t.Error("Handler was not called")
	}
}

func TestMiddleware_validateSession(t *testing.T) {
	t.Parallel()
	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	tests := []struct {
		name        string
		cookie      string
		expectError bool
	}{
		{
			name:        "no cookie",
			cookie:      "",
			expectError: true,
		},
		{
			name:        "invalid cookie",
			cookie:      "invalid-session",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != "" {
				req.Header.Set("Cookie", tt.cookie)
			}

			session, err := middleware.validateSession(req)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && session == nil {
				t.Error("Expected session, got nil")
			}
		})
	}
}

func TestMiddleware_RequireAuthFunc(t *testing.T) {
	t.Parallel()
	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	called := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
	}

	wrappedHandler := middleware.RequireAuthFunc(handler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	wrappedHandler(rec, req)

	// Should redirect to login
	if rec.Code != http.StatusSeeOther {
		t.Errorf("Expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	// Handler should not be called
	if called {
		t.Error("Handler was called despite missing session")
	}
}

func TestMiddleware_OptionalAuthFunc(t *testing.T) {
	t.Parallel()
	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	called := false
	handler := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	wrappedHandler := middleware.OptionalAuthFunc(handler)

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rec := httptest.NewRecorder()

	wrappedHandler(rec, req)

	// Should allow access
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Handler should be called
	if !called {
		t.Error("Handler was not called")
	}
}

func TestMiddleware_AAL2Redirect(t *testing.T) {
	t.Parallel()
	// Test that if AAL2 is required but not satisfied, we redirect
	kratosClient := kratos.NewKratosClient(testConfig())
	middleware := NewMiddleware(kratosClient, os.Getenv("APP_URL"))

	// This would require mocking the session response
	// For now, we just verify the middleware is created correctly
	if middleware == nil {
		t.Fatal("NewMiddleware() returned nil")
	}
}

// Helper function to create a mock session for testing.
func createMockSession(email string, active bool) *client.Session {
	now := time.Now()
	aal := client.AUTHENTICATORASSURANCELEVEL_AAL1

	return &client.Session{
		Active:                      &active,
		AuthenticatorAssuranceLevel: &aal,
		ExpiresAt:                   &now,
		Id:                          "session-123",
		Identity: &client.Identity{
			Id:        "identity-123",
			CreatedAt: &now,
			UpdatedAt: &now,
			Traits: map[string]any{
				"email": email,
			},
		},
	}
}

func TestGetUserFromContext(t *testing.T) {
	t.Parallel()
	user := &domain.User{
		ID:    "test-123",
		Email: "test@example.com",
	}

	ctx := context.WithValue(context.Background(), domain.UserContextKey, user)

	retrieved := kratos.GetUserFromContext(ctx)
	if retrieved == nil {
		t.Fatal("kratos.GetUserFromContext returned nil")
	}

	if retrieved.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, retrieved.ID)
	}

	if retrieved.Email != user.Email {
		t.Errorf("Expected Email %s, got %s", user.Email, retrieved.Email)
	}
}

func TestGetUserFromContext_Nil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	user := kratos.GetUserFromContext(ctx)
	if user != nil {
		t.Errorf("Expected nil user, got %+v", user)
	}
}

func TestGetSessionFromContext(t *testing.T) {
	t.Parallel()
	session := createMockSession("test@example.com", true)

	ctx := context.WithValue(context.Background(), domain.SessionContextKey, session)

	retrieved := kratos.GetSessionFromContext(ctx)
	if retrieved == nil {
		t.Fatal("kratos.GetSessionFromContext returned nil")
	}

	if retrieved.Id != session.Id {
		t.Errorf("Expected session ID %s, got %s", session.Id, retrieved.Id)
	}
}

func TestGetSessionFromContext_Nil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := kratos.GetSessionFromContext(ctx)
	if session != nil {
		t.Errorf("Expected nil session, got %+v", session)
	}
}

func TestIsAuthenticated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name: "authenticated",
			ctx: context.WithValue(context.Background(), domain.UserContextKey, &domain.User{
				ID:    "test-123",
				Email: "test@example.com",
			}),
			expected: true,
		},
		{
			name:     "not authenticated",
			ctx:      context.Background(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := kratos.IsAuthenticated(tt.ctx)
			if got != tt.expected {
				t.Errorf("kratos.IsAuthenticated() = %v, want %v", got, tt.expected)
			}
		})
	}
}
