package tenants

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestService_List(t *testing.T) {
	tenantID := uuid.New()
	mockResponse := TenantListResponse{
		Items: []TenantResponse{
			{
				ID:   tenantID,
				Name: "Test Tenant",
				Slug: "test-tenant",
			},
		},
		Total:    1,
		Page:     1,
		PageSize: 10,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL and query params
		expectedURL := "/api/v1/tenants?page=1&page_size=10"
		if r.URL.String() != expectedURL {
			t.Errorf("Expected URL %s, got %s", expectedURL, r.URL.String())
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.List(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Errorf("Expected 1 tenant, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != tenantID {
		t.Errorf("Expected tenant ID %s, got %s", tenantID, resp.Items[0].ID)
	}
}

func TestService_List_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	_, err := service.List(context.Background(), 1, 10)
	if err == nil {
		t.Fatal("Expected error for unauthorized request, got nil")
	}
	if err.Error() != "unauthorized" {
		t.Errorf("Expected error 'unauthorized', got '%v'", err)
	}
}

func TestService_Create(t *testing.T) {
	tenantID := uuid.New()
	mockRequest := CreateTenantRequest{
		Name: "New Org",
		Slug: "new-org",
	}
	mockResponse := TenantResponse{
		ID:   tenantID,
		Name: "New Org",
		Slug: "new-org",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL
		if r.URL.Path != "/api/v1/tenants" {
			t.Errorf("Expected path /api/v1/tenants, got %s", r.URL.Path)
		}

		// Verify Method
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		// Verify Body
		var req CreateTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if req.Name != mockRequest.Name {
			t.Errorf("Expected Name %s, got %s", mockRequest.Name, req.Name)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Create(context.Background(), &mockRequest)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if resp.ID != tenantID {
		t.Errorf("Expected tenant ID %s, got %s", tenantID, resp.ID)
	}
	if resp.Name != mockRequest.Name {
		t.Errorf("Expected Name %s, got %s", mockRequest.Name, resp.Name)
	}
}

func TestService_Get(t *testing.T) {
	tenantID := uuid.New()
	mockResponse := TenantResponse{
		ID:   tenantID,
		Name: "Test Org",
		Slug: "test-org",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/tenants/" + tenantID.String()
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Get(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if resp.ID != tenantID {
		t.Errorf("Expected tenant ID %s, got %s", tenantID, resp.ID)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	tenantID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	_, err := service.Get(context.Background(), tenantID)
	if err == nil {
		t.Fatal("Expected error for non-existent tenant, got nil")
	}
	if err.Error() != "tenant not found" {
		t.Errorf("Expected error 'tenant not found', got '%v'", err)
	}
}

func TestService_Update(t *testing.T) {
	tenantID := uuid.New()
	newName := "Updated Org"
	mockRequest := UpdateTenantRequest{
		Name: &newName,
	}
	mockResponse := TenantResponse{
		ID:   tenantID,
		Name: newName,
		Slug: "test-org",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Expected method PUT, got %s", r.Method)
		}

		var req UpdateTenantRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if *req.Name != newName {
			t.Errorf("Expected Name %s, got %s", newName, *req.Name)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.Update(context.Background(), tenantID, &mockRequest)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if resp.Name != newName {
		t.Errorf("Expected name %s, got %s", newName, resp.Name)
	}
}

func TestService_Delete(t *testing.T) {
	tenantID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Expected method DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	err := service.Delete(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
}
