//go:build integration
// +build integration

package auth

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUsername = "testuser"
	testPassword = "TestPass123!"
)

func TestService_Integration_LoginSuccess(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	resp, err := service.Login(ctx, &LoginRequest{
		Username: testUsername,
		Password: testPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.NotEmpty(t, resp.AccessToken)
	assert.Equal(t, "Bearer", resp.TokenType)
	assert.Greater(t, resp.ExpiresIn, int64(0))
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, testUsername, resp.User.Username)
	assert.NotEmpty(t, resp.User.UserID)
	assert.NotEmpty(t, resp.User.Email)
	assert.NotEmpty(t, resp.User.Role)
}

func TestService_Integration_LoginInvalidCredentials(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	_, err := service.Login(ctx, &LoginRequest{
		Username: "nonexistent",
		Password: "wrongpass",
	})
	assert.Error(t, err)
}

func TestService_Integration_RefreshToken(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	loginResp, err := service.Login(ctx, &LoginRequest{
		Username: testUsername,
		Password: testPassword,
	})
	require.NoError(t, err)

	refreshResp, err := service.Refresh(ctx, &RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	})
	require.NoError(t, err)
	require.NotNil(t, refreshResp)

	assert.NotEmpty(t, refreshResp.AccessToken)
	assert.Equal(t, "Bearer", refreshResp.TokenType)
	assert.Greater(t, refreshResp.ExpiresIn, int64(0))
}

func TestService_Integration_RefreshInvalidToken(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	_, err := service.Refresh(ctx, &RefreshTokenRequest{
		RefreshToken: "invalid-token",
	})
	assert.Error(t, err)
}

func TestService_Integration_GetMe(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	loginResp, err := service.Login(ctx, &LoginRequest{
		Username: testUsername,
		Password: testPassword,
	})
	require.NoError(t, err)

	meResp, err := service.GetMe(ctx, loginResp.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, meResp)

	assert.Equal(t, testUsername, meResp.User.Username)
	assert.NotEmpty(t, meResp.User.UserID)
	assert.NotEmpty(t, meResp.User.Email)
}

func TestService_Integration_GetMeUnauthorized(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	_, err := service.GetMe(ctx, "invalid-token")
	assert.Error(t, err)
}

func TestService_Integration_Logout(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	loginResp, err := service.Login(ctx, &LoginRequest{
		Username: testUsername,
		Password: testPassword,
	})
	require.NoError(t, err)

	err = service.Logout(ctx, loginResp.AccessToken, loginResp.RefreshToken)
	require.NoError(t, err)
}

func TestService_Integration_FullAuthFlow(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	// 1. Login
	loginResp, err := service.Login(ctx, &LoginRequest{
		Username: testUsername,
		Password: testPassword,
	})
	require.NoError(t, err)

	// 2. Get Me with access token
	meResp, err := service.GetMe(ctx, loginResp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, loginResp.User.UserID, meResp.User.UserID)

	// 3. Refresh
	refreshResp, err := service.Refresh(ctx, &RefreshTokenRequest{
		RefreshToken: loginResp.RefreshToken,
	})
	require.NoError(t, err)

	// 4. Get Me with new access token
	meResp2, err := service.GetMe(ctx, refreshResp.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, loginResp.User.UserID, meResp2.User.UserID)
}
