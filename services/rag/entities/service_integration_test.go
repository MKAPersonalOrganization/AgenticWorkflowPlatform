//go:build integration

package entities_test

import (
"context"
"os"
"testing"

"school-with-ai/domain"
"school-with-ai/services/rag/entities"

"github.com/google/uuid"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func baseURL(t *testing.T) string {
t.Helper()
u := os.Getenv("EDGEQUAKE_BASE_URL")
if u == "" {
t.Skip("EDGEQUAKE_BASE_URL not set")
}
return u
}

func tenantCtx() domain.TenantContext {
tid := uuid.MustParse("00000000-0000-0000-0000-000000000002")
wid := uuid.MustParse("00000000-0000-0000-0000-000000000003")
return domain.TenantContext{
TenantID:    &tid,
WorkspaceID: &wid,
UserID:      "00000000-0000-0000-0000-000000000001",
Role:        "admin",
}
}

func TestListEntities(t *testing.T) {
svc := entities.NewService(baseURL(t), nil)
resp, err := svc.ListEntities(context.Background(), tenantCtx(), nil)
require.NoError(t, err)
assert.NotNil(t, resp)
assert.GreaterOrEqual(t, resp.Total, 0)
assert.NotNil(t, resp.Items)
}

func TestEntityExists_NotFound(t *testing.T) {
svc := entities.NewService(baseURL(t), nil)
resp, err := svc.EntityExists(context.Background(), tenantCtx(), "NONEXISTENT")
require.NoError(t, err)
assert.NotNil(t, resp)
assert.False(t, resp.Exists)
}

func TestGetEntity_NotFound(t *testing.T) {
svc := entities.NewService(baseURL(t), nil)
_, err := svc.GetEntity(context.Background(), tenantCtx(), "NONEXISTENT")
require.Error(t, err)
assert.Contains(t, err.Error(), "not found")
}

func TestGetEntityNeighborhood_NotFound(t *testing.T) {
svc := entities.NewService(baseURL(t), nil)
_, err := svc.GetEntityNeighborhood(context.Background(), tenantCtx(), "NONEXISTENT", 1)
require.Error(t, err)
assert.Contains(t, err.Error(), "not found")
}

func TestCreateEntity_AndCleanup(t *testing.T) {
svc := entities.NewService(baseURL(t), nil)
tc := tenantCtx()

createReq := entities.CreateEntityRequest{
EntityName:  "INTEGRATION_TEST_ENTITY",
EntityType:  "TEST",
Description: "integration test",
SourceID:    "manual_entry",
}

resp, err := svc.CreateEntity(context.Background(), tc, createReq)
require.NoError(t, err)
require.NotNil(t, resp)
assert.NotEmpty(t, resp.Status)
assert.Equal(t, "INTEGRATION_TEST_ENTITY", resp.Entity.EntityName)

// Cleanup
_, _ = svc.DeleteEntity(context.Background(), tc, "INTEGRATION_TEST_ENTITY", true)
}
