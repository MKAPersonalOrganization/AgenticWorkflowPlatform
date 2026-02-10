package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/auth"
	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestParseWhitelistPatterns(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []WhitelistPattern
	}{
		{
			name:  "simple paths",
			input: "/health,/api/status",
			expected: []WhitelistPattern{
				{Path: "/health", IsPrefix: false},
				{Path: "/api/status", IsPrefix: false},
			},
		},
		{
			name:  "prefix patterns",
			input: "/api/*,/docs/*",
			expected: []WhitelistPattern{
				{Path: "/api", IsPrefix: true},
				{Path: "/docs", IsPrefix: true},
			},
		},
		{
			name:  "mixed patterns",
			input: "/health,/api/*",
			expected: []WhitelistPattern{
				{Path: "/health", IsPrefix: false},
				{Path: "/api", IsPrefix: true},
			},
		},
		{
			name:  "with spaces",
			input: " /health , /api/* ",
			expected: []WhitelistPattern{
				{Path: "/health", IsPrefix: false},
				{Path: "/api", IsPrefix: true},
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []WhitelistPattern{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseWhitelistPatterns(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPathWhitelisted(t *testing.T) {
	patterns := []WhitelistPattern{
		{Path: "/health", IsPrefix: false},
		{Path: "/api", IsPrefix: true},
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/api/query", true},
		{"/api", true},
		{"/api/documents/list", true},
		{"/protected", false},
		{"/healthcheck", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isPathWhitelisted(tt.path, patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldSkipTokenRenewal(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/documents/paginated", true},
		{"/documents/pipeline_status", true},
		{"/documents/pipeline_status/123", true},
		{"/query", false},
		{"/documents/upload", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := shouldSkipTokenRenewal(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAuthMiddleware_WhitelistedPath(t *testing.T) {
	cfg := &config.Config{
		APIKey:         "test-api-key",
		WhitelistPaths: "/health,/api/*",
		TokenSecret:    "test-secret",
	}
	authHandler := auth.NewAuthHandler(cfg)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_ValidAPIKey(t *testing.T) {
	cfg := &config.Config{
		APIKey:         "test-api-key",
		WhitelistPaths: "/health",
		TokenSecret:    "test-secret",
	}
	authHandler := auth.NewAuthHandler(cfg)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "test-api-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_InvalidAPIKey(t *testing.T) {
	cfg := &config.Config{
		APIKey:         "test-api-key",
		WhitelistPaths: "/health",
		TokenSecret:    "test-secret",
	}
	authHandler := auth.NewAuthHandler(cfg)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "wrong-api-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAuthMiddleware_ValidBearerToken(t *testing.T) {
	cfg := &config.Config{
		WhitelistPaths:   "/health",
		TokenSecret:      "test-secret-jwt",
		AuthAccounts:     "testuser:testpass",
		TokenExpireHours: 48,
	}
	authHandler := auth.NewAuthHandler(cfg)

	// Create a valid token
	token, err := authHandler.CreateToken("testuser", "user", nil, nil)
	require.NoError(t, err)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		username, _ := c.Get("username")
		c.JSON(http.StatusOK, gin.H{"username": username})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	cfg := &config.Config{
		WhitelistPaths:   "/health",
		TokenSecret:      "test-secret-jwt",
		AuthAccounts:     "testuser:testpass",
		TokenExpireHours: 48,
	}
	authHandler := auth.NewAuthHandler(cfg)

	// Create an expired token
	expiredHours := -1.0
	token, err := authHandler.CreateToken("testuser", "user", &expiredHours, nil)
	require.NoError(t, err)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	cfg := &config.Config{
		WhitelistPaths:   "/health",
		TokenSecret:      "test-secret-jwt",
		AuthAccounts:     "testuser:testpass",
		TokenExpireHours: 48,
	}
	authHandler := auth.NewAuthHandler(cfg)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_NoAuth(t *testing.T) {
	cfg := &config.Config{
		APIKey:         "test-api-key",
		WhitelistPaths: "/health",
		TokenSecret:    "test-secret",
		AuthAccounts:   "admin:pass",
	}
	authHandler := auth.NewAuthHandler(cfg)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_NoAuthConfigured(t *testing.T) {
	cfg := &config.Config{
		APIKey:         "", // No API key
		WhitelistPaths: "/health",
		TokenSecret:    "test-secret",
		AuthAccounts:   "", // No accounts
	}
	authHandler := auth.NewAuthHandler(cfg)

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should allow access when no auth is configured
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCORSMiddleware(t *testing.T) {
	cfg := &config.Config{
		CORSOrigins: "http://localhost:3000,http://example.com",
	}

	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("allowed origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("disallowed origin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "http://malicious.com")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("preflight request", func(t *testing.T) {
		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestCORSMiddleware_Wildcard(t *testing.T) {
	cfg := &config.Config{
		CORSOrigins: "*",
	}

	router := gin.New()
	router.Use(CORSMiddleware(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestGetWorkspaceFromRequest(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		workspace := GetWorkspaceFromRequest(c)
		c.JSON(http.StatusOK, gin.H{"workspace": workspace})
	})

	t.Run("from header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Workspace", "workspace-from-header")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "workspace-from-header")
	})

	t.Run("from query param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?workspace=workspace-from-query", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "workspace-from-query")
	})

	t.Run("header takes precedence", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?workspace=from-query", nil)
		req.Header.Set("X-Workspace", "from-header")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "from-header")
	})

	t.Run("empty when not provided", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"workspace":""`)
	})
}

func TestTokenAutoRenewal(t *testing.T) {
	cfg := &config.Config{
		WhitelistPaths:      "/health",
		TokenSecret:         "test-secret-jwt",
		AuthAccounts:        "testuser:testpass",
		TokenExpireHours:    1, // 1 hour expiry
		TokenAutoRenew:      true,
		TokenRenewThreshold: 0.9, // Renew when < 90% remaining (i.e., almost all tokens should renew)
	}
	authHandler := auth.NewAuthHandler(cfg)

	// Create a token that's about to expire (0.05 hours = 3 minutes from now)
	shortExpiry := 0.05
	token, err := authHandler.CreateToken("testuser", "user", &shortExpiry, nil)
	require.NoError(t, err)

	// Clear the renewal cache
	tokenRenewalCache.Lock()
	tokenRenewalCache.m = make(map[string]time.Time)
	tokenRenewalCache.Unlock()

	router := gin.New()
	router.Use(AuthMiddleware(cfg, authHandler))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// Should have a new token in the response header
	assert.NotEmpty(t, w.Header().Get("X-New-Token"))
}
