//go:build integration

package graph_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/shared"
	"school-with-ai/modules/knowledge/rag/graph"

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

func tenantCtx() shared.TenantContext {
	tid := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	wid := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return shared.TenantContext{
		TenantID:    &tid,
		WorkspaceID: &wid,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestGetGraph(t *testing.T) {
	svc := graph.NewService(baseURL(t), nil)
	resp, err := svc.GetGraph(context.Background(), tenantCtx(), nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.TotalNodes, 0)
	assert.GreaterOrEqual(t, resp.TotalEdges, 0)
}

func TestGetNode_NotFound(t *testing.T) {
	svc := graph.NewService(baseURL(t), nil)
	_, err := svc.GetNode(context.Background(), tenantCtx(), "non-existent-node-id")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSearchLabels(t *testing.T) {
	svc := graph.NewService(baseURL(t), nil)
	resp, err := svc.SearchLabels(context.Background(), tenantCtx(), "test", 0)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	// labels may be empty on a fresh DB
	assert.NotNil(t, resp.Labels)
}

func TestSearchNodes(t *testing.T) {
	svc := graph.NewService(baseURL(t), nil)
	resp, err := svc.SearchNodes(context.Background(), tenantCtx(), "test", 0)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.TotalMatches, 0)
}

func TestGetPopularLabels(t *testing.T) {
	svc := graph.NewService(baseURL(t), nil)
	resp, err := svc.GetPopularLabels(context.Background(), tenantCtx(), 0)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.TotalEntities, 0)
}

func TestGetDegreesBatch(t *testing.T) {
	svc := graph.NewService(baseURL(t), nil)
	resp, err := svc.GetDegreesBatch(context.Background(), tenantCtx(), []string{"node1", "node2"})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, resp.Count)
	assert.Len(t, resp.Degrees, 2)
}
