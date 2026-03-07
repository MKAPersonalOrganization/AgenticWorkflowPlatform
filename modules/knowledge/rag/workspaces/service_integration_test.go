//go:build integration
// +build integration

package workspaces

import (
	"context"
	"os"
	"testing"

	"school-with-ai/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestService_Integration_List(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	tCtx := &shared.TenantContext{
		TenantID: &tenantID,
	}

	resp, err := service.List(context.Background(), tCtx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 1, "Should have at least one workspace")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 1, "Should have at least one workspace")
}

func TestService_Integration_Create(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	tCtx := &shared.TenantContext{
		TenantID: &tenantID,
	}

	workspaceName := "Integration Test Workspace " + uuid.New().String()[:8]
	req := &CreateWorkspaceRequest{
		Name: workspaceName,
		Slug: stringPtr(uuid.New().String()[:8]),
	}

	resp, err := service.Create(context.Background(), tCtx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, workspaceName, resp.Name)
	assert.Equal(t, tenantID, resp.TenantID)
	assert.NotEmpty(t, resp.ID)
}

func TestService_Integration_CreateAndListFlow(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	tCtx := &shared.TenantContext{
		TenantID: &tenantID,
	}

	// 1. Create a workspace
	workspaceName := "Flow Test Workspace " + uuid.New().String()[:8]
	createReq := &CreateWorkspaceRequest{
		Name: workspaceName,
		Slug: stringPtr("flow-" + uuid.New().String()[:8]),
	}

	createResp, err := service.Create(context.Background(), tCtx, createReq)
	assert.NoError(t, err)
	assert.NotNil(t, createResp)
	assert.Equal(t, workspaceName, createResp.Name)

	// 2. List workspaces and check if the new one is there
	listResp, err := service.List(context.Background(), tCtx, 1, 100) // Large limit to ensure we see it
	assert.NoError(t, err)
	assert.NotNil(t, listResp)

	found := false
	for _, item := range listResp.Items {
		if item.ID == createResp.ID {
			found = true
			assert.Equal(t, workspaceName, item.Name)
			break
		}
	}
	assert.True(t, found, "Newly created workspace should be present in the list")
}

func TestService_Integration_Get(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	// Use the known default workspace ID
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	resp, err := service.Get(context.Background(), nil, workspaceID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, workspaceID, resp.ID)
	assert.Equal(t, "Default Workspace", resp.Name)
}

func TestService_Integration_Delete(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	tCtx := &shared.TenantContext{
		TenantID: &tenantID,
	}

	// 1. Create a workspace to delete
	workspaceName := "Delete Test Workspace " + uuid.New().String()[:8]
	createReq := &CreateWorkspaceRequest{
		Name: workspaceName,
		Slug: stringPtr("delete-" + uuid.New().String()[:8]),
	}

	createResp, err := service.Create(context.Background(), tCtx, createReq)
	assert.NoError(t, err)

	// 2. Delete it
	err = service.Delete(context.Background(), tCtx, createResp.ID)
	assert.NoError(t, err)

	// 3. Verify it's gone
	_, err = service.Get(context.Background(), tCtx, createResp.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workspace not found")
}

func TestService_Integration_GetStats(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	// Use the known default workspace ID
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	resp, err := service.GetStats(context.Background(), nil, workspaceID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, workspaceID, resp.WorkspaceID)
	// Expect at least some documents/entities if the test DB is populated,
	// but we'll just check that it parses without error.
	assert.GreaterOrEqual(t, resp.DocumentCount, 0)
}

func TestService_Integration_Update(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	tCtx := &shared.TenantContext{
		TenantID: &tenantID,
	}

	// 1. Create a workspace to update
	createReq := &CreateWorkspaceRequest{
		Name: "Update Test " + uuid.New().String()[:8],
		Slug: stringPtr("update-" + uuid.New().String()[:8]),
	}
	createResp, err := service.Create(context.Background(), tCtx, createReq)
	assert.NoError(t, err)

	// 2. Update it
	newName := "New Name " + uuid.New().String()[:8]
	updateReq := &UpdateWorkspaceRequest{
		Name: &newName,
	}
	updateResp, err := service.Update(context.Background(), tCtx, createResp.ID, updateReq)

	assert.NoError(t, err)
	assert.Equal(t, newName, updateResp.Name)

	// 3. Verify it's updated
	getResp, err := service.Get(context.Background(), tCtx, createResp.ID)
	assert.NoError(t, err)
	assert.Equal(t, newName, getResp.Name)
}

// TestService_Integration_WorkspaceResponseFields verifies all fields from the
// Rust WorkspaceResponse are properly deserialized including SPEC-032 and SPEC-040 fields.
func TestService_Integration_WorkspaceResponseFields(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	// Use the known default workspace ID
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")

	resp, err := service.Get(context.Background(), nil, workspaceID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, workspaceID, resp.ID)
	assert.Equal(t, tenantID, resp.TenantID)
	assert.NotEmpty(t, resp.Name)
	assert.NotEmpty(t, resp.Slug)
	assert.True(t, resp.IsActive)

	// SPEC-032: LLM configuration
	assert.NotEmpty(t, resp.LlmModel)
	assert.NotEmpty(t, resp.LlmProvider)
	assert.NotEmpty(t, resp.LlmFullID, "llm_full_id should be populated")

	// SPEC-032: Embedding configuration
	assert.NotEmpty(t, resp.EmbeddingModel)
	assert.NotEmpty(t, resp.EmbeddingProvider)
	assert.Greater(t, resp.EmbeddingDimension, 0)
	assert.NotEmpty(t, resp.EmbeddingFullID, "embedding_full_id should be populated")

	// Timestamps should be RFC3339 strings
	assert.NotEmpty(t, resp.CreatedAt)
	assert.NotEmpty(t, resp.UpdatedAt)
	assert.Contains(t, resp.CreatedAt, "T", "created_at should be an RFC3339 string")
}

// TestService_Integration_WorkspaceStatsFields verifies all fields from WorkspaceStatsResponse.
func TestService_Integration_WorkspaceStatsFields(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	resp, err := service.GetStats(context.Background(), nil, workspaceID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, workspaceID, resp.WorkspaceID)
	assert.GreaterOrEqual(t, resp.DocumentCount, 0)
	assert.GreaterOrEqual(t, resp.EntityCount, 0)
	assert.GreaterOrEqual(t, resp.RelationshipCount, 0)
	assert.GreaterOrEqual(t, resp.EntityTypeCount, 0)
	assert.GreaterOrEqual(t, resp.ChunkCount, 0)
	assert.GreaterOrEqual(t, resp.EmbeddingCount, 0)
}
