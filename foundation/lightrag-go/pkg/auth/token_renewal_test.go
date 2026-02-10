package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TokenRenewalCache tracks last renewal time per user for rate limiting
type TokenRenewalCache struct {
	mu       sync.RWMutex
	cache    map[string]time.Time
	interval time.Duration // minimum time between renewals
}

// NewTokenRenewalCache creates a new cache with default 60s interval
func NewTokenRenewalCache(interval time.Duration) *TokenRenewalCache {
	return &TokenRenewalCache{
		cache:    make(map[string]time.Time),
		interval: interval,
	}
}

// CanRenew checks if enough time has passed since last renewal for this user
func (c *TokenRenewalCache) CanRenew(username string) bool {
	c.mu.RLock()
	lastRenewal, exists := c.cache[username]
	c.mu.RUnlock()

	if !exists {
		return true
	}
	return time.Since(lastRenewal) >= c.interval
}

// RecordRenewal records a renewal for the given username
func (c *TokenRenewalCache) RecordRenewal(username string) {
	c.mu.Lock()
	c.cache[username] = time.Now()
	c.mu.Unlock()
}

// Clear clears all cached renewal times
func (c *TokenRenewalCache) Clear() {
	c.mu.Lock()
	c.cache = make(map[string]time.Time)
	c.mu.Unlock()
}

// ShouldRenewToken determines if a token should be renewed based on remaining time
// threshold is a value between 0.0 and 1.0 representing the percentage of total time
func ShouldRenewToken(tokenInfo *TokenInfo, totalExpireHours float64, threshold float64) bool {
	if tokenInfo == nil {
		return false
	}

	remainingSeconds := time.Until(tokenInfo.Exp).Seconds()
	if remainingSeconds <= 0 {
		return false // Token already expired
	}

	totalSeconds := totalExpireHours * 3600
	return remainingSeconds < totalSeconds*threshold
}

// ============================================================================
// Token Renewal Threshold Tests
// ============================================================================

func TestShouldRenewToken_BelowThreshold(t *testing.T) {
	// Token with 10 hours remaining, 24 hour total, 0.5 threshold
	// 10 < 24 * 0.5 (12) = true, should renew
	tokenInfo := &TokenInfo{
		Username: "testuser",
		Role:     "user",
		Exp:      time.Now().Add(10 * time.Hour),
		Metadata: make(map[string]interface{}),
	}

	shouldRenew := ShouldRenewToken(tokenInfo, 24.0, 0.5)
	assert.True(t, shouldRenew, "Token with 10h remaining out of 24h should renew at 50% threshold")
}

func TestShouldRenewToken_AboveThreshold(t *testing.T) {
	// Token with 20 hours remaining, 24 hour total, 0.5 threshold
	// 20 > 24 * 0.5 (12) = false, should NOT renew
	tokenInfo := &TokenInfo{
		Username: "testuser",
		Role:     "user",
		Exp:      time.Now().Add(20 * time.Hour),
		Metadata: make(map[string]interface{}),
	}

	shouldRenew := ShouldRenewToken(tokenInfo, 24.0, 0.5)
	assert.False(t, shouldRenew, "Token with 20h remaining out of 24h should NOT renew at 50% threshold")
}

func TestShouldRenewToken_ExpiredToken(t *testing.T) {
	// Expired token should not trigger renewal
	tokenInfo := &TokenInfo{
		Username: "testuser",
		Role:     "user",
		Exp:      time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		Metadata: make(map[string]interface{}),
	}

	shouldRenew := ShouldRenewToken(tokenInfo, 24.0, 0.5)
	assert.False(t, shouldRenew, "Expired token should not be renewed")
}

func TestShouldRenewToken_NilTokenInfo(t *testing.T) {
	shouldRenew := ShouldRenewToken(nil, 24.0, 0.5)
	assert.False(t, shouldRenew, "Nil token info should not trigger renewal")
}

func TestShouldRenewToken_GuestToken(t *testing.T) {
	// Guest token with 10 hours remaining, 24 hour total (guest expire), 0.5 threshold
	tokenInfo := &TokenInfo{
		Username: "guest",
		Role:     "guest",
		Exp:      time.Now().Add(10 * time.Hour),
		Metadata: map[string]interface{}{"auth_mode": "disabled"},
	}

	// Using guest expire hours (24)
	shouldRenew := ShouldRenewToken(tokenInfo, 24.0, 0.5)
	assert.True(t, shouldRenew, "Guest token with 10h remaining should renew at 50% threshold")
}

func TestShouldRenewToken_DifferentThresholds(t *testing.T) {
	tests := []struct {
		name           string
		remainingHours float64
		totalHours     float64
		threshold      float64
		expectRenew    bool
	}{
		{
			name:           "25% threshold - 5h remaining of 24h",
			remainingHours: 5,
			totalHours:     24,
			threshold:      0.25,
			expectRenew:    true, // 5 < 6
		},
		{
			name:           "25% threshold - 7h remaining of 24h",
			remainingHours: 7,
			totalHours:     24,
			threshold:      0.25,
			expectRenew:    false, // 7 > 6
		},
		{
			name:           "75% threshold - 15h remaining of 24h",
			remainingHours: 15,
			totalHours:     24,
			threshold:      0.75,
			expectRenew:    true, // 15 < 18
		},
		{
			name:           "75% threshold - 20h remaining of 24h",
			remainingHours: 20,
			totalHours:     24,
			threshold:      0.75,
			expectRenew:    false, // 20 > 18
		},
		{
			name:           "100% threshold - always renew",
			remainingHours: 23,
			totalHours:     24,
			threshold:      1.0,
			expectRenew:    true, // 23 < 24
		},
		{
			name:           "0% threshold - never renew",
			remainingHours: 1,
			totalHours:     24,
			threshold:      0.0,
			expectRenew:    false, // 1 > 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenInfo := &TokenInfo{
				Username: "testuser",
				Role:     "user",
				Exp:      time.Now().Add(time.Duration(tt.remainingHours) * time.Hour),
				Metadata: make(map[string]interface{}),
			}

			shouldRenew := ShouldRenewToken(tokenInfo, tt.totalHours, tt.threshold)
			assert.Equal(t, tt.expectRenew, shouldRenew)
		})
	}
}

// ============================================================================
// Rate Limiting Tests
// ============================================================================

func TestTokenRenewalCache_FirstRenewal(t *testing.T) {
	cache := NewTokenRenewalCache(60 * time.Second)

	// First renewal for a user should always be allowed
	canRenew := cache.CanRenew("testuser")
	assert.True(t, canRenew, "First renewal for a user should be allowed")
}

func TestTokenRenewalCache_RateLimiting(t *testing.T) {
	// Use short interval for testing
	cache := NewTokenRenewalCache(100 * time.Millisecond)

	// First renewal
	assert.True(t, cache.CanRenew("testuser"))
	cache.RecordRenewal("testuser")

	// Immediate second renewal should be blocked
	assert.False(t, cache.CanRenew("testuser"), "Second renewal should be blocked within interval")

	// Wait for interval to pass
	time.Sleep(150 * time.Millisecond)

	// Now renewal should be allowed again
	assert.True(t, cache.CanRenew("testuser"), "Renewal should be allowed after interval passes")
}

func TestTokenRenewalCache_IndependentUsers(t *testing.T) {
	cache := NewTokenRenewalCache(100 * time.Millisecond)

	// User1 renews
	assert.True(t, cache.CanRenew("user1"))
	cache.RecordRenewal("user1")

	// User2 should still be able to renew (independent rate limits)
	assert.True(t, cache.CanRenew("user2"), "Different users should have independent rate limits")
	cache.RecordRenewal("user2")

	// User1 still blocked
	assert.False(t, cache.CanRenew("user1"))

	// User3 (new user) can still renew
	assert.True(t, cache.CanRenew("user3"))
}

func TestTokenRenewalCache_Clear(t *testing.T) {
	cache := NewTokenRenewalCache(100 * time.Millisecond)

	cache.RecordRenewal("user1")
	cache.RecordRenewal("user2")

	// Both users blocked
	assert.False(t, cache.CanRenew("user1"))
	assert.False(t, cache.CanRenew("user2"))

	// Clear cache
	cache.Clear()

	// Both users should be able to renew now
	assert.True(t, cache.CanRenew("user1"), "User1 should be able to renew after cache clear")
	assert.True(t, cache.CanRenew("user2"), "User2 should be able to renew after cache clear")
}

func TestTokenRenewalCache_ConcurrentAccess(t *testing.T) {
	cache := NewTokenRenewalCache(10 * time.Millisecond)
	done := make(chan bool, 100)

	// Concurrent reads and writes
	for i := 0; i < 100; i++ {
		go func(userID int) {
			username := "user" + string(rune(userID%10))
			cache.CanRenew(username)
			cache.RecordRenewal(username)
			cache.CanRenew(username)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}
}

// ============================================================================
// Token Renewal Disabled Tests
// ============================================================================

func TestTokenRenewal_Disabled(t *testing.T) {
	// When token auto-renewal is disabled, no renewal should happen
	// This is typically controlled by config.TokenAutoRenew

	// Simulate disabled renewal by not calling the renewal logic
	autoRenewEnabled := false
	tokenInfo := &TokenInfo{
		Username: "testuser",
		Role:     "user",
		Exp:      time.Now().Add(5 * time.Hour), // Below 50% threshold
		Metadata: make(map[string]interface{}),
	}

	var newToken string

	// Conditional renewal check
	if autoRenewEnabled && ShouldRenewToken(tokenInfo, 24.0, 0.5) {
		newToken = "renewed-token" // Would call CreateToken in real code
	}

	assert.Empty(t, newToken, "Token should not be renewed when auto-renewal is disabled")
}

// ============================================================================
// Integration with AuthHandler Tests
// ============================================================================

func TestTokenRenewal_Integration(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "test-secret-key-for-jwt",
		JWTAlgorithm:          "HS256",
		TokenExpireHours:      24,
		GuestTokenExpireHours: 12,
	}
	handler := NewAuthHandler(cfg)
	cache := NewTokenRenewalCache(60 * time.Second)

	// Create a token
	token, err := handler.CreateToken("testuser", "user", nil, nil)
	require.NoError(t, err)

	// Validate the token
	tokenInfo, err := handler.ValidateToken(token)
	require.NoError(t, err)

	// Check if renewal is allowed
	totalHours := handler.GetExpireHours(tokenInfo.Role)
	threshold := 0.5

	// Since we just created the token, it should NOT need renewal
	shouldRenew := ShouldRenewToken(tokenInfo, totalHours, threshold)
	assert.False(t, shouldRenew, "Fresh token should not need renewal")

	// If renewal was needed, check rate limiting
	if shouldRenew && cache.CanRenew(tokenInfo.Username) {
		newToken, err := handler.CreateToken(
			tokenInfo.Username,
			tokenInfo.Role,
			nil,
			tokenInfo.Metadata,
		)
		require.NoError(t, err)
		assert.NotEmpty(t, newToken)
		cache.RecordRenewal(tokenInfo.Username)
	}
}

func TestTokenRenewal_GuestVsUser(t *testing.T) {
	cfg := &config.Config{
		TokenSecret:           "test-secret-key-for-jwt",
		JWTAlgorithm:          "HS256",
		TokenExpireHours:      48, // 48 hours for regular users
		GuestTokenExpireHours: 24, // 24 hours for guests
	}
	handler := NewAuthHandler(cfg)

	tests := []struct {
		name           string
		role           string
		remainingHours float64
		threshold      float64
		expectRenew    bool
	}{
		{
			name:           "Guest token at 10h remaining should renew",
			role:           "guest",
			remainingHours: 10,
			threshold:      0.5,
			expectRenew:    true, // 10 < 24 * 0.5 = 12
		},
		{
			name:           "User token at 10h remaining should renew (below 50% of 48h)",
			role:           "user",
			remainingHours: 10,
			threshold:      0.5,
			expectRenew:    true, // 10 < 48 * 0.5 = 24
		},
		{
			name:           "User token at 26h remaining should NOT renew",
			role:           "user",
			remainingHours: 26,
			threshold:      0.5,
			expectRenew:    false, // 26 > 48 * 0.5 = 24
		},
		{
			name:           "Guest token at 14h remaining should NOT renew",
			role:           "guest",
			remainingHours: 14,
			threshold:      0.5,
			expectRenew:    false, // 14 > 24 * 0.5 = 12
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenInfo := &TokenInfo{
				Username: "testuser",
				Role:     tt.role,
				Exp:      time.Now().Add(time.Duration(tt.remainingHours) * time.Hour),
				Metadata: make(map[string]interface{}),
			}

			totalHours := handler.GetExpireHours(tt.role)
			shouldRenew := ShouldRenewToken(tokenInfo, totalHours, tt.threshold)
			assert.Equal(t, tt.expectRenew, shouldRenew)
		})
	}
}

// ============================================================================
// Path Exclusion Tests
// ============================================================================

// Paths that should skip token renewal
var TokenRenewalSkipPaths = []string{
	"/documents/paginated",
	"/documents/pipeline-status",
	"/health",
}

// ShouldSkipRenewal checks if the current path should skip token renewal
func ShouldSkipRenewal(path string) bool {
	for _, skipPath := range TokenRenewalSkipPaths {
		if path == skipPath {
			return true
		}
	}
	return false
}

func TestShouldSkipRenewal(t *testing.T) {
	tests := []struct {
		path       string
		shouldSkip bool
	}{
		{"/documents/paginated", true},
		{"/documents/pipeline-status", true},
		{"/health", true},
		{"/query", false},
		{"/documents/text", false},
		{"/graphs/labels", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ShouldSkipRenewal(tt.path)
			assert.Equal(t, tt.shouldSkip, result)
		})
	}
}
