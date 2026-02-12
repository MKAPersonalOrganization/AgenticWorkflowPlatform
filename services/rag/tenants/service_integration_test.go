//go:build integration
// +build integration

package tenants

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestService_Integration_List(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	resp, err := service.List(context.Background(), 1, 10)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, 1, "Should have at least one tenant")
	assert.NotEmpty(t, resp.Items)
}

func TestService_Integration_Create(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	tenantName := "Integration Org " + uuid.New().String()[:8]
	req := &CreateTenantRequest{
		Name: tenantName,
		Slug: "integration-org-" + uuid.New().String()[:8],
	}

	resp, err := service.Create(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, tenantName, resp.Name)
	assert.NotEmpty(t, resp.ID)
}

func TestService_Integration_Get(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	// 1. First list to get a valid ID
	listResp, err := service.List(context.Background(), 1, 1)
	assert.NoError(t, err)
	assert.NotEmpty(t, listResp.Items)

	tenantID := listResp.Items[0].ID

	// 2. Get that tenant
	resp, err := service.Get(context.Background(), tenantID)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, tenantID, resp.ID)
}

func TestService_Integration_Update(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	// 1. Create a tenant to update
	createReq := &CreateTenantRequest{
		Name: "Update Test Org " + uuid.New().String()[:8],
	}
	tenant, err := service.Create(context.Background(), createReq)
	assert.NoError(t, err)

	// 2. Update it
	newName := "Updated Integration Org"
	updateReq := &UpdateTenantRequest{
		Name: &newName,
	}
	resp, err := service.Update(context.Background(), tenant.ID, updateReq)

	assert.NoError(t, err)
	assert.Equal(t, newName, resp.Name)
}

func TestService_Integration_Delete(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)

	// 1. Create a tenant to delete
	createReq := &CreateTenantRequest{
		Name: "Delete Test Org " + uuid.New().String()[:8],
	}
	tenant, err := service.Create(context.Background(), createReq)
	assert.NoError(t, err)

	// 2. Delete it
	err = service.Delete(context.Background(), tenant.ID)
	assert.NoError(t, err)

	// 3. Verify it's gone
	_, err = service.Get(context.Background(), tenant.ID)
	assert.Error(t, err)
}

func TestService_Integration_Lifecycle(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	// 1. Create
	name := "Lifecycle Org " + uuid.New().String()[:8]
	createReq := &CreateTenantRequest{
		Name: name,
		Slug: "lifecycle-org-" + uuid.New().String()[:8],
	}
	tenant, err := service.Create(ctx, createReq)
	assert.NoError(t, err)
	assert.NotNil(t, tenant)
	tenantID := tenant.ID

	// 2. List & Verify presence
	listResp, err := service.List(ctx, 1, 100)
	assert.NoError(t, err)
	found := false
	for _, item := range listResp.Items {
		if item.ID == tenantID {
			found = true
			break
		}
	}
	assert.True(t, found, "Tenant should be in the list")

	// 3. Get & Verify details
	getResp, err := service.Get(ctx, tenantID)
	assert.NoError(t, err)
	assert.Equal(t, name, getResp.Name)

	// 4. Update
	newName := "Updated Lifecycle Org"
	updateReq := &UpdateTenantRequest{
		Name: &newName,
	}
	updateResp, err := service.Update(ctx, tenantID, updateReq)
	assert.NoError(t, err)
	assert.Equal(t, newName, updateResp.Name)

	// 5. Delete
	err = service.Delete(ctx, tenantID)
	assert.NoError(t, err)

	// 6. Verify Deletion
	_, err = service.Get(ctx, tenantID)
	assert.Error(t, err, "Get should fail after deletion")
}
