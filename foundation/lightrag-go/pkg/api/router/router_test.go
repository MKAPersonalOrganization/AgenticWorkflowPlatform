package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewRouter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = true

	router := NewRouter(cfg)
	require.NotNil(t, router)
	require.NotNil(t, router.Engine())
}

func TestRouter_HealthEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = true

	router := NewRouter(cfg)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.Engine().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "healthy", resp["status"])
}

func TestRouter_AuthEndpoints(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = false
	cfg.AuthAccounts = "testuser:testpass" // This enables auth

	router := NewRouter(cfg)

	t.Run("login endpoint exists", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/auth/login", nil)
		w := httptest.NewRecorder()
		router.Engine().ServeHTTP(w, req)

		// Should return bad request due to missing body, not 404
		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})

	t.Run("guest-login endpoint exists", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/auth/guest-login", nil)
		w := httptest.NewRecorder()
		router.Engine().ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusNotFound, w.Code)
	})
}

func TestRouter_QueryEndpointsRequireAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = false
	cfg.AuthAccounts = "admin:password" // This enables auth

	router := NewRouter(cfg)

	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/query"},
		{"POST", "/query/stream"},
		{"POST", "/retrieval"},
		{"POST", "/query_data"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.Engine().ServeHTTP(w, req)

			// Should return 401 without auth token
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestRouter_DocumentEndpointsRequireAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = false
	cfg.AuthAccounts = "admin:password" // This enables auth

	router := NewRouter(cfg)

	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/documents/text"},
		{"POST", "/documents/texts"},
		{"POST", "/documents/file"},
		{"POST", "/documents/batch"},
		{"POST", "/documents/scan"},
		{"GET", "/documents"},
		{"GET", "/documents/123"},
		{"DELETE", "/documents/123"},
		{"DELETE", "/documents"},
		{"DELETE", "/documents/all"},
		{"GET", "/documents/pipeline-status"},
		{"POST", "/documents/index-all"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.Engine().ServeHTTP(w, req)

			// Should return 401 without auth token
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestRouter_GraphEndpointsRequireAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = false
	cfg.AuthAccounts = "admin:password" // This enables auth

	router := NewRouter(cfg)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/graphs/labels"},
		{"GET", "/graphs/test"},
		{"GET", "/graphs/test/text-chunks"},
		{"PUT", "/graphs/entities/123"},
		{"POST", "/graphs/entities/merge"},
		{"DELETE", "/graphs/entities/123"},
		{"DELETE", "/graphs/relations/123"},
		{"POST", "/graphs/relations"},
		{"DELETE", "/graphs/nodes/orphan"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.Engine().ServeHTTP(w, req)

			// Should return 401 without auth token
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestRouter_OllamaEndpointsNoAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = false
	cfg.AuthAccounts = "admin:password" // This enables auth

	router := NewRouter(cfg)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/version"},
		{"GET", "/api/tags"},
		{"GET", "/api/ps"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			router.Engine().ServeHTTP(w, req)

			// Ollama endpoints should not require auth
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRouter_WithAuthDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = true

	router := NewRouter(cfg)

	// Query endpoint should work without auth when auth is disabled
	req := httptest.NewRequest("GET", "/documents", nil)
	w := httptest.NewRecorder()
	router.Engine().ServeHTTP(w, req)

	// Should return 200 (or some non-401 status)
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
}

func TestRouter_CORSHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuthDisabled = true
	cfg.CORSAllowedOrigins = []string{"http://localhost:3000"}

	router := NewRouter(cfg)

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	router.Engine().ServeHTTP(w, req)

	// Check CORS headers are set
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Origin"))
}
