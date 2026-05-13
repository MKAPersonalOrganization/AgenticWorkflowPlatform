package folders

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestService_List(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	mockResponse := []FolderResponse{
		{ID: uuid.New(), TenantID: tenantID, Name: "Folder 1", Position: 0, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"},
		{ID: uuid.New(), TenantID: tenantID, Name: "Folder 2", Position: 1, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected method GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(resp) != 2 {
		t.Errorf("Expected 2 items, got %d", len(resp))
	}
}

func TestService_Create(t *testing.T) {
	t.Parallel()

	folderID := uuid.New()
	tenantID := uuid.New()
	mockRequest := CreateFolderRequest{Name: "New Folder"}
	mockResponse := FolderResponse{ID: folderID, TenantID: tenantID, Name: "New Folder", Position: 0, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Create(context.Background(), nil, &mockRequest)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if resp.ID != folderID {
		t.Errorf("Expected ID %v, got %v", folderID, resp.ID)
	}
}

func TestService_Update(t *testing.T) {
	t.Parallel()

	folderID := uuid.New()
	tenantID := uuid.New()
	updatedName := "Updated Folder"
	mockRequest := UpdateFolderRequest{Name: &updatedName}
	mockResponse := FolderResponse{ID: folderID, TenantID: tenantID, Name: "Updated Folder", Position: 0, CreatedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:00Z"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("Expected method PATCH, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Update(context.Background(), nil, folderID, &mockRequest)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if resp.Name != "Updated Folder" {
		t.Errorf("Expected name Updated Folder, got %s", resp.Name)
	}
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	folderID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Expected method DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	err := service.Delete(context.Background(), nil, folderID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}
