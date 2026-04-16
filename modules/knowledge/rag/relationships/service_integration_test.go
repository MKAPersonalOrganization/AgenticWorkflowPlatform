//go:build integration

package relationships_test

import (
	"context"
	"os"
	"testing"

	"aicompass/modules/knowledge/tenant"
	"aicompass/modules/knowledge/rag/relationships"

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

func tenantCtx() tenant.TenantContext {
	tid := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	wid := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return tenant.TenantContext{
		TenantID:    &tid,
		WorkspaceID: &wid,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestListRelationships(t *testing.T) {
	svc := relationships.NewService(baseURL(t), nil)
	resp, err := svc.ListRelationships(context.Background(), tenantCtx(), nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 0)
	assert.NotNil(t, resp.Items)
}

func TestGetRelationship_NotFound(t *testing.T) {
	svc := relationships.NewService(baseURL(t), nil)
	_, err := svc.GetRelationship(context.Background(), tenantCtx(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
