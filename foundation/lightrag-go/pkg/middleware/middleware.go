// Package middleware provides HTTP middleware for the LightRAG API.
package middleware

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/auth"
	"github.com/lightrag/lightrag-go/pkg/config"
)

// TokenRenewalSkipPaths contains paths that should NOT trigger token auto-renewal.
var TokenRenewalSkipPaths = []string{
	"/health",
	"/documents/paginated",
	"/documents/pipeline_status",
}

// TokenRenewalMinInterval is the minimum interval between token renewals for the same user.
const TokenRenewalMinInterval = 60 * time.Second

// tokenRenewalCache tracks last renewal time per user.
var tokenRenewalCache = struct {
	sync.RWMutex
	m map[string]time.Time
}{m: make(map[string]time.Time)}

// WhitelistPattern represents a path matching pattern.
type WhitelistPattern struct {
	Path     string
	IsPrefix bool
}

// ParseWhitelistPatterns parses the whitelist paths configuration.
func ParseWhitelistPatterns(whitelistPaths string) []WhitelistPattern {
	patterns := []WhitelistPattern{}
	for _, path := range strings.Split(whitelistPaths, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if strings.HasSuffix(path, "/*") {
			patterns = append(patterns, WhitelistPattern{
				Path:     path[:len(path)-2],
				IsPrefix: true,
			})
		} else {
			patterns = append(patterns, WhitelistPattern{
				Path:     path,
				IsPrefix: false,
			})
		}
	}
	return patterns
}

// isPathWhitelisted checks if a path matches any whitelist pattern.
func isPathWhitelisted(path string, patterns []WhitelistPattern) bool {
	for _, pattern := range patterns {
		if pattern.IsPrefix && strings.HasPrefix(path, pattern.Path) {
			return true
		}
		if !pattern.IsPrefix && path == pattern.Path {
			return true
		}
	}
	return false
}

// shouldSkipTokenRenewal checks if the path should skip token renewal.
func shouldSkipTokenRenewal(path string) bool {
	for _, skipPath := range TokenRenewalSkipPaths {
		if path == skipPath || strings.HasPrefix(path, skipPath+"/") {
			return true
		}
	}
	return false
}

// AuthMiddlewareHandler wraps auth middleware with a Handler method.
type AuthMiddlewareHandler struct {
	authHandler *auth.AuthHandler
	cfg         *config.Config
	handler     gin.HandlerFunc
}

// NewAuthMiddleware creates a new auth middleware handler.
func NewAuthMiddleware(authHandler *auth.AuthHandler, cfg *config.Config) *AuthMiddlewareHandler {
	m := &AuthMiddlewareHandler{
		authHandler: authHandler,
		cfg:         cfg,
	}
	m.handler = AuthMiddleware(cfg, authHandler)
	return m
}

// Handler returns the gin handler function for this middleware.
func (m *AuthMiddlewareHandler) Handler() gin.HandlerFunc {
	return m.handler
}

// CORSMiddlewareHandler wraps CORS middleware with a Handler method.
type CORSMiddlewareHandler struct {
	cfg     *config.Config
	handler gin.HandlerFunc
}

// NewCORSMiddleware creates a new CORS middleware handler.
func NewCORSMiddleware(cfg *config.Config) *CORSMiddlewareHandler {
	m := &CORSMiddlewareHandler{
		cfg: cfg,
	}
	m.handler = CORSMiddleware(cfg)
	return m
}

// Handler returns the gin handler function for this middleware.
func (m *CORSMiddlewareHandler) Handler() gin.HandlerFunc {
	return m.handler
}

// AuthMiddleware creates authentication middleware.
func AuthMiddleware(cfg *config.Config, authHandler *auth.AuthHandler) gin.HandlerFunc {
	apiKey := cfg.APIKey
	whitelistPatterns := ParseWhitelistPatterns(cfg.WhitelistPaths)
	apiKeyConfigured := apiKey != ""
	authConfigured := authHandler.HasAccounts()
	authDisabled := cfg.AuthDisabled

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 1. If auth is disabled, allow all
		if authDisabled {
			c.Next()
			return
		}

		// 2. Check if path is whitelisted
		if isPathWhitelisted(path, whitelistPatterns) {
			c.Next()
			return
		}

		// 3. Try API Key authentication first
		if apiKeyConfigured {
			if apiKeyHeader := c.GetHeader("X-API-Key"); apiKeyHeader != "" {
				if apiKeyHeader == apiKey {
					c.Next()
					return
				}
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "Invalid API Key"})
				return
			}
		}

		// 3. Try Bearer token authentication
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			tokenInfo, err := authHandler.ValidateToken(token)
			if err != nil {
				if err == auth.ErrTokenExpired {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Token expired"})
				} else {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Invalid token"})
				}
				return
			}

			// Store token info in context
			c.Set("username", tokenInfo.Username)
			c.Set("role", tokenInfo.Role)
			c.Set("token_info", tokenInfo)

			// Token auto-renewal logic
			if cfg.TokenAutoRenew && !shouldSkipTokenRenewal(path) {
				handleTokenRenewal(c, authHandler, tokenInfo, cfg)
			}

			c.Next()
			return
		}

		// 4. If no valid authentication found
		if apiKeyConfigured || authConfigured {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Authentication required"})
			return
		}

		// No authentication configured, allow access
		c.Next()
	}
}

// handleTokenRenewal handles automatic token renewal.
func handleTokenRenewal(c *gin.Context, authHandler *auth.AuthHandler, tokenInfo *auth.TokenInfo, cfg *config.Config) {
	now := time.Now()
	remaining := tokenInfo.Exp.Sub(now)

	// Get total expiration duration based on role
	totalHours := authHandler.GetExpireHours(tokenInfo.Role)
	totalDuration := time.Duration(totalHours * float64(time.Hour))

	// Check if remaining time is less than threshold
	threshold := time.Duration(float64(totalDuration) * cfg.TokenRenewThreshold)
	if remaining < threshold {
		// Rate limiting check
		tokenRenewalCache.RLock()
		lastRenewal, exists := tokenRenewalCache.m[tokenInfo.Username]
		tokenRenewalCache.RUnlock()

		if !exists || now.Sub(lastRenewal) >= TokenRenewalMinInterval {
			// Create new token
			newToken, err := authHandler.CreateToken(
				tokenInfo.Username,
				tokenInfo.Role,
				nil,
				tokenInfo.Metadata,
			)
			if err != nil {
				log.Printf("Token auto-renew failed: %v", err)
				return
			}

			// Set new token in response header
			c.Header("X-New-Token", newToken)

			// Update renewal cache
			tokenRenewalCache.Lock()
			tokenRenewalCache.m[tokenInfo.Username] = now
			tokenRenewalCache.Unlock()

			log.Printf("Token auto-renewed for user %s (role: %s, remaining: %.0fs)",
				tokenInfo.Username, tokenInfo.Role, remaining.Seconds())
		}
	}
}

// CORSMiddleware creates CORS middleware.
func CORSMiddleware(cfg *config.Config) gin.HandlerFunc {
	origins := strings.Split(cfg.CORSOrigins, ",")

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowOrigin := ""
		for _, o := range origins {
			o = strings.TrimSpace(o)
			if o == "*" {
				allowOrigin = "*"
				break
			}
			if o == origin {
				allowOrigin = origin
				break
			}
		}

		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key, X-Workspace")
			c.Header("Access-Control-Expose-Headers", "X-New-Token")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}

		// Handle preflight
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RequestLoggerMiddleware logs incoming requests.
func RequestLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if raw != "" {
			path = path + "?" + raw
		}

		log.Printf("[GIN] %3d | %13v | %15s | %-7s %s",
			statusCode,
			latency,
			clientIP,
			method,
			path,
		)
	}
}

// RecoveryMiddleware handles panics.
func RecoveryMiddleware() gin.HandlerFunc {
	return gin.Recovery()
}

// GetWorkspaceFromRequest extracts workspace from request headers or query params.
func GetWorkspaceFromRequest(c *gin.Context) string {
	// First check header
	if workspace := c.GetHeader("X-Workspace"); workspace != "" {
		return workspace
	}
	// Then check query param
	if workspace := c.Query("workspace"); workspace != "" {
		return workspace
	}
	return ""
}
