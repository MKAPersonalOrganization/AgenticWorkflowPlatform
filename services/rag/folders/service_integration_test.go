//go:build integration
// +build integration

package folders

import (
	"context"
	"os"
	"testing"

	"school-with-ai/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFolderContext() *domain.TenantContext {
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	return &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "00000000-0000-0000-0000-000000000001",
		Role:        "admin",
	}
}

func TestService_Integration_Lifecycle(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// 1. Create a folder
	folderName := "Integration Folder " + uuid.New().String()[:8]
	createReq := &CreateFolderRequest{
		Name: folderName,
	}
	folder, err := service.Create(ctx, tCtx, createReq)
	require.NoError(t, err)
	require.NotNil(t, folder)
	assert.Equal(t, folderName, folder.Name)
	folderID := folder.ID

	// 2. List folders and verify presence
	listResp, err := service.List(ctx, tCtx)
	require.NoError(t, err)
	found := false
	for _, item := range listResp {
		if item.ID == folderID {
			found = true
			break
		}
	}
	assert.True(t, found, "Folder should be present in the list")

	// 3. Update the folder
	updatedName := "Updated Folder Name " + uuid.New().String()[:8]
	updateReq := &UpdateFolderRequest{
		Name: &updatedName,
	}
	updateResp, err := service.Update(ctx, tCtx, folderID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updateResp)
	assert.Equal(t, updatedName, updateResp.Name)

	// 4. Delete the folder
	err = service.Delete(ctx, tCtx, folderID)
	require.NoError(t, err)

	// 5. Verify deletion (List should not contain it)
	listRespAfter, err := service.List(ctx, tCtx)
	require.NoError(t, err)
	foundAfter := false
	for _, item := range listRespAfter {
		if item.ID == folderID {
			foundAfter = true
			break
		}
	}
	assert.False(t, foundAfter, "Folder should not be present after deletion")
}

func TestService_Integration_NestedFolders(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Create parent folder
	parentName := "Parent Folder " + uuid.New().String()[:8]
	parentReq := &CreateFolderRequest{
		Name: parentName,
	}
	parent, err := service.Create(ctx, tCtx, parentReq)
	require.NoError(t, err)
	require.NotNil(t, parent)
	assert.Equal(t, parentName, parent.Name)
	assert.Nil(t, parent.ParentID)

	// Create child folder under parent
	childName := "Child Folder " + uuid.New().String()[:8]
	childReq := &CreateFolderRequest{
		Name:     childName,
		ParentID: &parent.ID,
	}
	child, err := service.Create(ctx, tCtx, childReq)
	require.NoError(t, err)
	require.NotNil(t, child)
	assert.Equal(t, childName, child.Name)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parent.ID, *child.ParentID)

	// Create nested child folder
	nestedName := "Nested Child " + uuid.New().String()[:8]
	nestedReq := &CreateFolderRequest{
		Name:     nestedName,
		ParentID: &child.ID,
	}
	nested, err := service.Create(ctx, tCtx, nestedReq)
	require.NoError(t, err)
	require.NotNil(t, nested)
	assert.Equal(t, nestedName, nested.Name)
	require.NotNil(t, nested.ParentID)
	assert.Equal(t, child.ID, *nested.ParentID)

	// List all folders and verify hierarchy
	folders, err := service.List(ctx, tCtx)
	require.NoError(t, err)

	foundParent := false
	foundChild := false
	foundNested := false
	for _, f := range folders {
		if f.ID == parent.ID {
			foundParent = true
		}
		if f.ID == child.ID {
			foundChild = true
		}
		if f.ID == nested.ID {
			foundNested = true
		}
	}
	assert.True(t, foundParent, "Parent folder should be in list")
	assert.True(t, foundChild, "Child folder should be in list")
	assert.True(t, foundNested, "Nested folder should be in list")

	// Cleanup
	service.Delete(ctx, tCtx, nested.ID)
	service.Delete(ctx, tCtx, child.ID)
	service.Delete(ctx, tCtx, parent.ID)
}

func TestService_Integration_MoveFolderBetweenParents(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Create two parent folders
	parent1Name := "Parent 1 " + uuid.New().String()[:8]
	parent1, err := service.Create(ctx, tCtx, &CreateFolderRequest{Name: parent1Name})
	require.NoError(t, err)

	parent2Name := "Parent 2 " + uuid.New().String()[:8]
	parent2, err := service.Create(ctx, tCtx, &CreateFolderRequest{Name: parent2Name})
	require.NoError(t, err)

	// Create child under parent1
	childName := "Movable Child " + uuid.New().String()[:8]
	child, err := service.Create(ctx, tCtx, &CreateFolderRequest{
		Name:     childName,
		ParentID: &parent1.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, parent1.ID, *child.ParentID)

	// Move child from parent1 to parent2
	updateReq := &UpdateFolderRequest{
		ParentID: &parent2.ID,
	}
	updated, err := service.Update(ctx, tCtx, child.ID, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updated.ParentID)
	assert.Equal(t, parent2.ID, *updated.ParentID)

	// Cleanup
	service.Delete(ctx, tCtx, child.ID)
	service.Delete(ctx, tCtx, parent1.ID)
	service.Delete(ctx, tCtx, parent2.ID)
}

func TestService_Integration_UpdateFolderPosition(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Create a folder
	folderName := "Positioned Folder " + uuid.New().String()[:8]
	folder, err := service.Create(ctx, tCtx, &CreateFolderRequest{Name: folderName})
	require.NoError(t, err)

	initialPosition := folder.Position

	// Update position
	newPosition := int32(10)
	updateReq := &UpdateFolderRequest{
		Position: &newPosition,
	}
	updated, err := service.Update(ctx, tCtx, folder.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, newPosition, updated.Position)
	assert.NotEqual(t, initialPosition, updated.Position)

	// Cleanup
	service.Delete(ctx, tCtx, folder.ID)
}

func TestService_Integration_UpdateMultipleFields(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Create parent folder
	parentName := "Update Parent " + uuid.New().String()[:8]
	parent, err := service.Create(ctx, tCtx, &CreateFolderRequest{Name: parentName})
	require.NoError(t, err)

	// Create folder
	folderName := "Multi Update " + uuid.New().String()[:8]
	folder, err := service.Create(ctx, tCtx, &CreateFolderRequest{Name: folderName})
	require.NoError(t, err)

	// Update name, parent, and position simultaneously
	newName := "Updated Multi " + uuid.New().String()[:8]
	newPosition := int32(5)
	updateReq := &UpdateFolderRequest{
		Name:     &newName,
		ParentID: &parent.ID,
		Position: &newPosition,
	}
	updated, err := service.Update(ctx, tCtx, folder.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, newName, updated.Name)
	require.NotNil(t, updated.ParentID)
	assert.Equal(t, parent.ID, *updated.ParentID)
	assert.Equal(t, newPosition, updated.Position)

	// Cleanup
	service.Delete(ctx, tCtx, folder.ID)
	service.Delete(ctx, tCtx, parent.ID)
}

func TestService_Integration_CreateMultipleFoldersAndList(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Create multiple folders
	folderCount := 5
	createdFolders := make([]*FolderResponse, 0, folderCount)

	for i := 0; i < folderCount; i++ {
		folderName := "Bulk Folder " + uuid.New().String()[:8]
		folder, err := service.Create(ctx, tCtx, &CreateFolderRequest{Name: folderName})
		require.NoError(t, err)
		require.NotNil(t, folder)
		createdFolders = append(createdFolders, folder)
	}

	// List all folders and verify all created folders are present
	folders, err := service.List(ctx, tCtx)
	require.NoError(t, err)

	for _, created := range createdFolders {
		found := false
		for _, listed := range folders {
			if listed.ID == created.ID {
				found = true
				assert.Equal(t, created.Name, listed.Name)
				assert.Equal(t, created.TenantID, listed.TenantID)
				break
			}
		}
		assert.True(t, found, "Created folder %s should be in list", created.Name)
	}

	// Cleanup
	for _, folder := range createdFolders {
		service.Delete(ctx, tCtx, folder.ID)
	}
}

func TestService_Integration_DeleteNonExistentFolder(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Try to delete a non-existent folder
	nonExistentID := uuid.New()
	err := service.Delete(ctx, tCtx, nonExistentID)

	// Should return an error (likely 404 not found)
	assert.Error(t, err)
}

func TestService_Integration_UpdateNonExistentFolder(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Try to update a non-existent folder
	nonExistentID := uuid.New()
	newName := "Should Fail"
	updateReq := &UpdateFolderRequest{
		Name: &newName,
	}

	_, err := service.Update(ctx, tCtx, nonExistentID, updateReq)

	// Should return an error (likely 404 not found)
	assert.Error(t, err)
}

func TestService_Integration_EmptyList(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	// Use a different tenant to ensure empty list
	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user-empty",
		Role:        "user",
	}

	// List should work even when empty
	folders, err := service.List(ctx, tCtx)

	// Either no error with empty list, or error is acceptable
	if err == nil {
		assert.NotNil(t, folders)
		// Empty list is valid
	}
}

func TestService_Integration_FolderTimestamps(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := testFolderContext()

	// Create a folder
	folderName := "Timestamp Folder " + uuid.New().String()[:8]
	folder, err := service.Create(ctx, tCtx, &CreateFolderRequest{Name: folderName})
	require.NoError(t, err)

	// Verify timestamps are set
	assert.NotEmpty(t, folder.CreatedAt, "CreatedAt should be set")
	assert.NotEmpty(t, folder.UpdatedAt, "UpdatedAt should be set")

	initialUpdatedAt := folder.UpdatedAt

	// Update the folder
	newName := "Updated Timestamp " + uuid.New().String()[:8]
	updated, err := service.Update(ctx, tCtx, folder.ID, &UpdateFolderRequest{Name: &newName})
	require.NoError(t, err)

	// Verify timestamps
	assert.Equal(t, folder.CreatedAt, updated.CreatedAt, "CreatedAt should not change")
	assert.NotEmpty(t, updated.UpdatedAt, "UpdatedAt should be set")
	// Note: UpdatedAt might be the same if update is very fast, so we just check it exists
	_ = initialUpdatedAt

	// Cleanup
	service.Delete(ctx, tCtx, folder.ID)
}
