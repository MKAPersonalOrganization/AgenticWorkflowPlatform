//go:build integration
// +build integration

package provenance_test

import (
	"context"
	"os"
	"testing"

	"aicompass/shared"
	"aicompass/modules/knowledge/rag/provenance"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func testTenantContext() *shared.TenantContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return &shared.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestGetEntityProvenance_NotFound(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	svc := provenance.NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testTenantContext()

	_, err := svc.GetEntityProvenance(ctx, tCtx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
