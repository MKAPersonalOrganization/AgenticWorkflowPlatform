package auth

import (
	"testing"
	"time"

	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuthHandler(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "test-secret",
		JWTAlgorithm:          "HS256",
		TokenExpireHours:      48,
		GuestTokenExpireHours: 24,
		AuthAccounts:          "admin:admin123,user:user456",
	}

	handler := NewAuthHandler(cfg)

	assert.Equal(t, "test-secret", handler.secret)
	assert.Equal(t, "HS256", handler.algorithm)
	assert.Equal(t, 48.0, handler.expireHours)
	assert.Equal(t, 24.0, handler.guestExpireHours)
	assert.Len(t, handler.accounts, 2)
	assert.Equal(t, "admin123", handler.accounts["admin"])
	assert.Equal(t, "user456", handler.accounts["user"])
}

func TestNewAuthHandler_NoAccounts(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "test-secret",
		JWTAlgorithm:          "HS256",
		TokenExpireHours:      48,
		GuestTokenExpireHours: 24,
		AuthAccounts:          "",
	}

	handler := NewAuthHandler(cfg)

	assert.False(t, handler.HasAccounts())
	assert.Empty(t, handler.accounts)
}

func TestHasAccounts(t *testing.T) {
	t.Run("with accounts", func(t *testing.T) {
		cfg := &config.Config{
			TokenSecret:  "test-secret",
			AuthAccounts: "admin:admin123",
		}
		handler := NewAuthHandler(cfg)
		assert.True(t, handler.HasAccounts())
	})

	t.Run("without accounts", func(t *testing.T) {
		cfg := &config.Config{
			TokenSecret:  "test-secret",
			AuthAccounts: "",
		}
		handler := NewAuthHandler(cfg)
		assert.False(t, handler.HasAccounts())
	})
}

func TestValidateCredentials(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:  "test-secret",
		AuthAccounts: "admin:admin123,user:user456",
	}
	handler := NewAuthHandler(cfg)

	tests := []struct {
		name     string
		username string
		password string
		expected bool
	}{
		{"valid admin", "admin", "admin123", true},
		{"valid user", "user", "user456", true},
		{"wrong password", "admin", "wrongpass", false},
		{"unknown user", "unknown", "password", false},
		{"empty credentials", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.ValidateCredentials(tt.username, tt.password)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateToken(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "test-secret-key-for-jwt",
		JWTAlgorithm:          "HS256",
		TokenExpireHours:      48,
		GuestTokenExpireHours: 24,
	}
	handler := NewAuthHandler(cfg)

	t.Run("regular user token", func(t *testing.T) {
		token, err := handler.CreateToken("testuser", "user", nil, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate the token
		info, err := handler.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "testuser", info.Username)
		assert.Equal(t, "user", info.Role)
		assert.NotNil(t, info.Metadata)
	})

	t.Run("guest token with shorter expiry", func(t *testing.T) {
		token, err := handler.CreateToken("guest-user", "guest", nil, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		info, err := handler.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "guest", info.Role)

		// Guest tokens should expire in ~24 hours (give some tolerance)
		expectedExpiry := time.Now().Add(24 * time.Hour)
		assert.WithinDuration(t, expectedExpiry, info.Exp, 5*time.Minute)
	})

	t.Run("token with custom expiry", func(t *testing.T) {
		customHours := 1.0
		token, err := handler.CreateToken("shortuser", "user", &customHours, nil)
		require.NoError(t, err)

		info, err := handler.ValidateToken(token)
		require.NoError(t, err)

		expectedExpiry := time.Now().Add(1 * time.Hour)
		assert.WithinDuration(t, expectedExpiry, info.Exp, 5*time.Minute)
	})

	t.Run("token with metadata", func(t *testing.T) {
		metadata := map[string]interface{}{
			"ip":       "192.168.1.1",
			"browser":  "Chrome",
			"loggedAt": "2024-01-01",
		}
		token, err := handler.CreateToken("metauser", "user", nil, metadata)
		require.NoError(t, err)

		info, err := handler.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "192.168.1.1", info.Metadata["ip"])
		assert.Equal(t, "Chrome", info.Metadata["browser"])
	})
}

func TestValidateToken(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "test-secret-key-for-jwt",
		JWTAlgorithm:          "HS256",
		TokenExpireHours:      48,
		GuestTokenExpireHours: 24,
	}
	handler := NewAuthHandler(cfg)

	t.Run("valid token", func(t *testing.T) {
		token, err := handler.CreateToken("testuser", "admin", nil, map[string]interface{}{"key": "value"})
		require.NoError(t, err)

		info, err := handler.ValidateToken(token)
		require.NoError(t, err)
		assert.Equal(t, "testuser", info.Username)
		assert.Equal(t, "admin", info.Role)
		assert.Equal(t, "value", info.Metadata["key"])
	})

	t.Run("invalid token format", func(t *testing.T) {
		_, err := handler.ValidateToken("invalid.token.format")
		assert.Error(t, err)
		assert.Equal(t, ErrInvalidToken, err)
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := handler.ValidateToken("")
		assert.Error(t, err)
	})

	t.Run("tampered token", func(t *testing.T) {
		token, err := handler.CreateToken("testuser", "user", nil, nil)
		require.NoError(t, err)

		// Tamper with the token
		tamperedToken := token + "tampered"
		_, err = handler.ValidateToken(tamperedToken)
		assert.Error(t, err)
	})

	t.Run("wrong secret", func(t *testing.T) {
		token, err := handler.CreateToken("testuser", "user", nil, nil)
		require.NoError(t, err)

		// Create handler with different secret
		otherHandler := NewAuthHandler(&config.Config{
			TokenSecret: "different-secret",
		})

		_, err = otherHandler.ValidateToken(token)
		assert.Error(t, err)
	})

	t.Run("expired token", func(t *testing.T) {
		// Create a token with very short expiry
		negativeHours := -1.0 // Already expired
		token, err := handler.CreateToken("expireduser", "user", &negativeHours, nil)
		require.NoError(t, err)

		_, err = handler.ValidateToken(token)
		assert.Error(t, err)
		assert.Equal(t, ErrTokenExpired, err)
	})
}

func TestGetExpireHours(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "test-secret",
		TokenExpireHours:      48,
		GuestTokenExpireHours: 24,
	}
	handler := NewAuthHandler(cfg)

	assert.Equal(t, 48.0, handler.GetExpireHours("user"))
	assert.Equal(t, 48.0, handler.GetExpireHours("admin"))
	assert.Equal(t, 24.0, handler.GetExpireHours("guest"))
}

func TestInitDefaultAuthHandler(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "global-secret",
		JWTAlgorithm:          "HS256",
		TokenExpireHours:      48,
		GuestTokenExpireHours: 24,
		AuthAccounts:          "testadmin:testpass",
	}

	InitDefaultAuthHandler(cfg)

	require.NotNil(t, DefaultAuthHandler)
	assert.True(t, DefaultAuthHandler.HasAccounts())
	assert.True(t, DefaultAuthHandler.ValidateCredentials("testadmin", "testpass"))
}

func TestConcurrentTokenValidation(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:      "test-secret-concurrent",
		TokenExpireHours: 48,
	}
	handler := NewAuthHandler(cfg)

	token, err := handler.CreateToken("concurrent-user", "user", nil, nil)
	require.NoError(t, err)

	// Run concurrent validations
	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			info, err := handler.ValidateToken(token)
			assert.NoError(t, err)
			assert.Equal(t, "concurrent-user", info.Username)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}
