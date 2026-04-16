//go:build integration
// +build integration

package lineage_test

import (
	"context"
	"os"
	"testing"

	"aicompass/modules/knowledge/tenant"
	"aicompass/modules/knowledge/rag/lineage"

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

func tenantCtx() *tenant.TenantContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return &tenant.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestGetEntityLineage_NotFound(t *testing.T) {
	svc := lineage.NewService(baseURL(t), nil)
	_, err := svc.GetEntityLineage(context.Background(), tenantCtx(), "NONEXISTENT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetDocumentLineage_NotFound(t *testing.T) {
	svc := lineage.NewService(baseURL(t), nil)
	_, err := svc.GetDocumentLineage(context.Background(), tenantCtx(), "00000000-0000-0000-0000-000000000099")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
