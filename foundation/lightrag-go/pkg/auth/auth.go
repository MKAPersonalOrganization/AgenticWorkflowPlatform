// Package auth provides JWT authentication handling for LightRAG API.
package auth

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lightrag/lightrag-go/pkg/config"
)

// ErrTokenExpired indicates the token has expired.
var ErrTokenExpired = errors.New("token expired")

// ErrInvalidToken indicates the token is invalid.
var ErrInvalidToken = errors.New("invalid token")

// TokenPayload represents the JWT token payload.
type TokenPayload struct {
	Sub      string                 `json:"sub"`      // Username
	Exp      int64                  `json:"exp"`      // Expiration time (Unix timestamp)
	Role     string                 `json:"role"`     // User role
	Metadata map[string]interface{} `json:"metadata"` // Additional metadata
}

// TokenInfo represents validated token information.
type TokenInfo struct {
	Username string
	Role     string
	Metadata map[string]interface{}
	Exp      time.Time
}

// AuthHandler handles JWT authentication.
type AuthHandler struct {
	secret           string
	algorithm        string
	expireHours      float64
	guestExpireHours float64
	accounts         map[string]string
	mu               sync.RWMutex
}

// NewAuthHandler creates a new AuthHandler from configuration.
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	handler := &AuthHandler{
		secret:           cfg.TokenSecret,
		algorithm:        cfg.JWTAlgorithm,
		expireHours:      cfg.TokenExpireHours,
		guestExpireHours: cfg.GuestTokenExpireHours,
		accounts:         make(map[string]string),
	}

	// Parse auth accounts
	if cfg.AuthAccounts != "" {
		for _, account := range strings.Split(cfg.AuthAccounts, ",") {
			parts := strings.SplitN(account, ":", 2)
			if len(parts) == 2 {
				handler.accounts[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	return handler
}

// HasAccounts returns true if authentication is configured.
func (h *AuthHandler) HasAccounts() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.accounts) > 0
}

// ValidateCredentials validates username and password.
func (h *AuthHandler) ValidateCredentials(username, password string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if storedPassword, ok := h.accounts[username]; ok {
		return storedPassword == password
	}
	return false
}

// CreateToken creates a new JWT token.
func (h *AuthHandler) CreateToken(username, role string, customExpireHours *float64, metadata map[string]interface{}) (string, error) {
	// Choose expiration time based on role
	var expireHours float64
	if customExpireHours != nil {
		expireHours = *customExpireHours
	} else if role == "guest" {
		expireHours = h.guestExpireHours
	} else {
		expireHours = h.expireHours
	}

	expire := time.Now().Add(time.Duration(expireHours * float64(time.Hour)))

	// Create claims
	claims := jwt.MapClaims{
		"sub":  username,
		"exp":  expire.Unix(),
		"role": role,
	}

	if metadata != nil {
		claims["metadata"] = metadata
	} else {
		claims["metadata"] = map[string]interface{}{}
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign and return
	return token.SignedString([]byte(h.secret))
}

// ValidateToken validates a JWT token and returns token information.
func (h *AuthHandler) ValidateToken(tokenString string) (*TokenInfo, error) {
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(h.secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	// Extract expiration
	expFloat, ok := claims["exp"].(float64)
	if !ok {
		return nil, ErrInvalidToken
	}
	expTime := time.Unix(int64(expFloat), 0)

	// Check if expired
	if time.Now().After(expTime) {
		return nil, ErrTokenExpired
	}

	// Extract username
	username, ok := claims["sub"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	// Extract role (default to "user")
	role := "user"
	if r, ok := claims["role"].(string); ok {
		role = r
	}

	// Extract metadata
	metadata := make(map[string]interface{})
	if m, ok := claims["metadata"].(map[string]interface{}); ok {
		metadata = m
	}

	return &TokenInfo{
		Username: username,
		Role:     role,
		Metadata: metadata,
		Exp:      expTime,
	}, nil
}

// GetExpireHours returns the expiration hours for a given role.
func (h *AuthHandler) GetExpireHours(role string) float64 {
	if role == "guest" {
		return h.guestExpireHours
	}
	return h.expireHours
}

// DefaultAuthHandler is the default global auth handler.
var DefaultAuthHandler *AuthHandler

// InitDefaultAuthHandler initializes the default auth handler.
func InitDefaultAuthHandler(cfg *config.Config) {
	DefaultAuthHandler = NewAuthHandler(cfg)
}
