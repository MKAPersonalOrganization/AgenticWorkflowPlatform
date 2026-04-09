package workspaces

import (
	"aicompass/shared"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestService_List(t *testing.T) {
	tenantID := uuid.New()
	tCtx := &shared.TenantContext{
		TenantID: &tenantID,
	}

	mockResponse := WorkspaceListResponse{
		Items: []WorkspaceResponse{
			{
				ID:   uuid.New(),
				Name: "Test Workspace",
				Slug: "test",
			},
		},
		Total: 1,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, fmt.Sprintf("/api/v1/tenants/%s/workspaces", tenantID.String()), r.URL.Path)
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Equal(t, "0", r.URL.Query().Get("offset"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.List(context.Background(), tCtx, 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 1, resp.Total)
	assert.Equal(t, "Test Workspace", resp.Items[0].Name)
}

func TestService_Create(t *testing.T) {
	tenantID := uuid.New()
	tCtx := &shared.TenantContext{
		TenantID: &tenantID,
	}

	reqBody := &CreateWorkspaceRequest{
		Name: "New Workspace",
		Slug: stringPtr("new-workspace"),
	}

	mockResponse := WorkspaceResponse{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     "New Workspace",
		Slug:     "new-workspace",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, fmt.Sprintf("/api/v1/tenants/%s/workspaces", tenantID.String()), r.URL.Path)

		var receivedReq CreateWorkspaceRequest
		err := json.NewDecoder(r.Body).Decode(&receivedReq)
		assert.NoError(t, err)
		assert.Equal(t, reqBody.Name, receivedReq.Name)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Create(context.Background(), tCtx, reqBody)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, mockResponse.Name, resp.Name)
}

func TestService_Get(t *testing.T) {
	workspaceID := uuid.New()
	mockResponse := WorkspaceResponse{
		ID:   workspaceID,
		Name: "Get Test Workspace",
		Slug: "get-test",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, fmt.Sprintf("/api/v1/workspaces/%s", workspaceID.String()), r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Get(context.Background(), nil, workspaceID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, mockResponse.Name, resp.Name)
	assert.Equal(t, workspaceID, resp.ID)
}

func TestService_Delete(t *testing.T) {
	workspaceID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, fmt.Sprintf("/api/v1/workspaces/%s", workspaceID.String()), r.URL.Path)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	err := service.Delete(context.Background(), nil, workspaceID)

	assert.NoError(t, err)
}

func TestService_GetStats(t *testing.T) {
	workspaceID := uuid.New()
	mockResponse := WorkspaceStatsResponse{
		WorkspaceID:       workspaceID,
		DocumentCount:     10,
		EntityCount:       50,
		RelationshipCount: 25,
		EntityTypeCount:   5,
		ChunkCount:        100,
		EmbeddingCount:    80,
		StorageBytes:      1024 * 1024,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, fmt.Sprintf("/api/v1/workspaces/%s/stats", workspaceID.String()), r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.GetStats(context.Background(), nil, workspaceID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, mockResponse.DocumentCount, resp.DocumentCount)
	assert.Equal(t, workspaceID, resp.WorkspaceID)
}

func TestService_Update(t *testing.T) {
	workspaceID := uuid.New()
	newName := "Updated Name"
	req := &UpdateWorkspaceRequest{
		Name: &newName,
	}

	mockResponse := WorkspaceResponse{
		ID:   workspaceID,
		Name: newName,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, fmt.Sprintf("/api/v1/workspaces/%s", workspaceID.String()), r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Update(context.Background(), nil, workspaceID, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, newName, resp.Name)
}
