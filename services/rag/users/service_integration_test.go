//go:build integration
// +build integration

package users

import (
"context"
"os"
"testing"

"github.com/google/uuid"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestService_Integration_UserLifecycle(t *testing.T) {
baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
if baseURL == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}

service := NewService(baseURL, nil)
ctx := context.Background()

// 1. Create a user
suffix := uuid.New().String()[:8]
role := "user"
createReq := &CreateUserRequest{
Username: "inttest-" + suffix,
Email:    "inttest-" + suffix + "@example.com",
Password: "IntTestPass1!",
Role:     &role,
}
createResp, err := service.Create(ctx, createReq)
require.NoError(t, err)
require.NotNil(t, createResp)
assert.Equal(t, createReq.Username, createResp.User.Username)
assert.Equal(t, createReq.Email, createResp.User.Email)
assert.Equal(t, "user", createResp.User.Role)
assert.NotEmpty(t, createResp.User.UserID)
assert.NotEmpty(t, createResp.CreatedAt)
userID := createResp.User.UserID

// 2. Get user by ID
getResp, err := service.Get(ctx, userID)
require.NoError(t, err)
require.NotNil(t, getResp)
assert.Equal(t, createReq.Username, getResp.Username)
assert.Equal(t, createReq.Email, getResp.Email)
assert.Equal(t, "user", getResp.Role)

// 3. Delete user
err = service.Delete(ctx, userID)
require.NoError(t, err)

// 4. Verify deletion – Get should fail
_, err = service.Get(ctx, userID)
assert.Error(t, err)
}

func TestService_Integration_ListUsersReturnsValidStructure(t *testing.T) {
baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
if baseURL == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}

service := NewService(baseURL, nil)
ctx := context.Background()

// The list endpoint is currently a stub (returns empty), but should
// still return a valid paginated structure.
page := 1
pageSize := 5
params := &ListUsersParams{
Page:     &page,
PageSize: &pageSize,
}

resp, err := service.List(ctx, params)
require.NoError(t, err)
require.NotNil(t, resp)
assert.Equal(t, page, resp.Page)
assert.Equal(t, pageSize, resp.PageSize)
assert.GreaterOrEqual(t, resp.Total, 0)
assert.GreaterOrEqual(t, resp.TotalPages, 0)
assert.NotNil(t, resp.Users)
}

func TestService_Integration_GetNonExistentUser(t *testing.T) {
baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
if baseURL == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}

service := NewService(baseURL, nil)
ctx := context.Background()

_, err := service.Get(ctx, uuid.New().String())
assert.Error(t, err)
}

func TestService_Integration_DeleteNonExistentUser(t *testing.T) {
baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
if baseURL == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}

service := NewService(baseURL, nil)
ctx := context.Background()

err := service.Delete(ctx, uuid.New().String())
assert.Error(t, err)
}

func TestService_Integration_CreateDuplicateUser(t *testing.T) {
baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
if baseURL == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}

service := NewService(baseURL, nil)
ctx := context.Background()

suffix := uuid.New().String()[:8]
role := "user"
createReq := &CreateUserRequest{
Username: "duptest-" + suffix,
Email:    "duptest-" + suffix + "@example.com",
Password: "DupTestPass1!",
Role:     &role,
}

// Create user first time
resp, err := service.Create(ctx, createReq)
require.NoError(t, err)
require.NotNil(t, resp)
userID := resp.User.UserID

// Attempt duplicate creation – should fail
_, err = service.Create(ctx, createReq)
assert.Error(t, err)

// Cleanup
_ = service.Delete(ctx, userID)
}
