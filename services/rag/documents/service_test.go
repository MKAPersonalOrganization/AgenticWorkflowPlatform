package documents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"school-with-ai/domain"

	"github.com/google/uuid"
)

func TestService_Upload(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	userID := "test-user"
	role := "admin"

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      userID,
		Role:        role,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL
		if r.URL.Path != "/api/v1/documents/upload" {
			t.Errorf("Expected path /api/v1/documents/upload, got %s", r.URL.Path)
		}

		// Verify Method
		if r.Method != http.MethodPost {
			t.Errorf("Expected method POST, got %s", r.Method)
		}

		// Verify Headers
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
		}
		if r.Header.Get("X-Workspace-ID") != workspaceID.String() {
			t.Errorf("Expected X-Workspace-ID %s, got %s", workspaceID.String(), r.Header.Get("X-Workspace-ID"))
		}
		if r.Header.Get("X-User-ID") != userID {
			t.Errorf("Expected X-User-ID %s, got %s", userID, r.Header.Get("X-User-ID"))
		}
		if r.Header.Get("X-User-Role") != role {
			t.Errorf("Expected X-User-Role %s, got %s", role, r.Header.Get("X-User-Role"))
		}

		// Verify Multipart Content
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Errorf("Failed to parse multipart form: %v", err)
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("file is required: %v", err)
		}
		defer file.Close()

		if header.Filename != "test.txt" {
			t.Errorf("Expected filename test.txt, got %s", header.Filename)
		}

		content, _ := io.ReadAll(file)
		if string(content) != "hello world" {
			t.Errorf("Expected content 'hello world', got '%s'", string(content))
		}

		// Respond with success
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(FileUploadResponse{
			DocumentID: "doc-123",
		})
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	content := strings.NewReader("hello world")

	resp, err := service.Upload(context.Background(), tCtx, "test.txt", "text/plain", content)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if resp.DocumentID != "doc-123" {
		t.Errorf("Expected document ID doc-123, got %s", resp.DocumentID)
	}
}

func TestService_List_Headers(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	userID := "test-user"
	role := "admin"

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      userID,
		Role:        role,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL and query params
		expectedURL := "/api/v1/documents?page=1&page_size=10"
		if r.URL.String() != expectedURL {
			t.Errorf("Expected URL %s, got %s", expectedURL, r.URL.String())
		}

		// Verify Headers
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
		}
		if r.Header.Get("X-Workspace-ID") != workspaceID.String() {
			t.Errorf("Expected X-Workspace-ID %s, got %s", workspaceID.String(), r.Header.Get("X-Workspace-ID"))
		}
		if r.Header.Get("X-User-ID") != userID {
			t.Errorf("Expected X-User-ID %s, got %s", userID, r.Header.Get("X-User-ID"))
		}
		if r.Header.Get("X-User-Role") != role {
			t.Errorf("Expected X-User-Role %s, got %s", role, r.Header.Get("X-User-Role"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListDocumentsResponse{
			Documents: []DocumentSummary{},
		})
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	_, err := service.List(context.Background(), tCtx, 1, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
}

func TestService_UploadAndListFlow(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
	}

	docs := []DocumentSummary{
		{ID: "doc-1", FileName: stringPtr("test.txt")},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(FileUploadResponse{DocumentID: "doc-1"})
		} else {
			json.NewEncoder(w).Encode(ListDocumentsResponse{Documents: docs})
		}
	}))
	defer server.Close()

	service := NewService(server.URL, nil)

	// Upload
	uploadResp, err := service.Upload(context.Background(), tCtx, "test.txt", "text/plain", strings.NewReader("content"))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// List
	listResp, err := service.List(context.Background(), tCtx, 1, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listResp.Documents) != 1 || listResp.Documents[0].ID != uploadResp.DocumentID {
		t.Errorf("List did not return the uploaded document")
	}
}

func TestService_ListPDFs(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
		Role:        "admin",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/documents/pdf" {
			t.Errorf("Expected path /api/v1/documents/pdf, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("status") != "completed" {
			t.Errorf("Expected status completed, got %s", r.URL.Query().Get("status"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListPdfsResponse{
			Items: []PdfListItem{
				{PdfID: "pdf-123", Filename: "test.pdf", Status: "completed"},
			},
		})
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.ListPDFs(context.Background(), tCtx, 1, 10, "completed")
	if err != nil {
		t.Fatalf("ListPDFs failed: %v", err)
	}

	if len(resp.Items) != 1 || resp.Items[0].PdfID != "pdf-123" {
		t.Errorf("Unexpected ListPDFs response")
	}
}

func TestService_DeletePDF(t *testing.T) {
	pdfID := uuid.New().String()
	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/api/v1/documents/pdf/%s", pdfID)
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("Expected method DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	err := service.DeletePDF(context.Background(), tCtx, pdfID)
	if err != nil {
		t.Fatalf("DeletePDF failed: %v", err)
	}
}

func TestService_DeletePDF_NotFound(t *testing.T) {
	pdfID := "non-existent"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("PDF not found"))
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	err := service.DeletePDF(context.Background(), &domain.TenantContext{}, pdfID)
	if err != nil {
		t.Fatalf("DeletePDF should be idempotent and return no error for NotFound, got %v", err)
	}
}

func TestService_GetPDFProgress(t *testing.T) {
	trackID := uuid.New().String()
	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/api/v1/documents/pdf/progress/%s", trackID)
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PdfUploadProgress{
			TrackID:    trackID,
			IsComplete: true,
		})
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	resp, err := service.GetPDFProgress(context.Background(), tCtx, trackID)
	if err != nil {
		t.Fatalf("GetPDFProgress failed: %v", err)
	}

	if resp.TrackID != trackID || !resp.IsComplete {
		t.Errorf("Unexpected GetPDFProgress response")
	}
}

func TestService_GetPDFProgress_NotFound(t *testing.T) {
	trackID := "non-existent"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	_, err := service.GetPDFProgress(context.Background(), &domain.TenantContext{}, trackID)
	if err == nil {
		t.Fatal("Expected error for non-existent Progress, got nil")
	}
}

func TestService_GetPDFStatus(t *testing.T) {
	pdfID := uuid.New().String()
	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/api/v1/documents/pdf/%s", pdfID)
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PdfStatusResponse{
			PdfID:  pdfID,
			Status: "completed",
			Metadata: PdfStatusMetadata{
				Filename: "test.pdf",
			},
		})
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	status, err := service.GetPDFStatus(context.Background(), tCtx, pdfID)
	if err != nil {
		t.Fatalf("GetPDFStatus failed: %v", err)
	}

	if status.PdfID != pdfID {
		t.Errorf("Expected PdfID %s, got %s", pdfID, status.PdfID)
	}
	if status.Status != "completed" {
		t.Errorf("Expected Status completed, got %s", status.Status)
	}
	if status.Metadata.Filename != "test.pdf" {
		t.Errorf("Expected Filename test.pdf, got %s", status.Metadata.Filename)
	}
}

func TestService_GetPDFStatus_NotFound(t *testing.T) {
	pdfID := "non-existent"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	_, err := service.GetPDFStatus(context.Background(), &domain.TenantContext{}, pdfID)
	if err == nil {
		t.Fatal("Expected error for non-existent PDF, got nil")
	}
}

func TestService_GetPDFContent(t *testing.T) {
	pdfID := uuid.New().String()
	tenantID := uuid.New()
	workspaceID := uuid.New()

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
		Role:        "admin",
	}

	contentStr := "# Markdown Content"
	mockContent := PdfContentResponse{
		PdfID:           pdfID,
		Filename:        "test.pdf",
		FileSizeBytes:   1024,
		ContentType:     "application/pdf",
		MarkdownContent: &contentStr,
		IsProcessed:     true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/api/v1/documents/pdf/%s/content", pdfID)
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockContent)
	}))
	defer server.Close()

	service := NewService(server.URL, nil)
	content, err := service.GetPDFContent(context.Background(), tCtx, pdfID)
	if err != nil {
		t.Fatalf("GetPDFContent failed: %v", err)
	}

	if content.PdfID != pdfID {
		t.Errorf("Expected PdfID %s, got %s", pdfID, content.PdfID)
	}
	if content.MarkdownContent == nil || *content.MarkdownContent != contentStr {
		t.Errorf("Expected markdown content %s, got %v", contentStr, content.MarkdownContent)
	}
}

func stringPtr(s string) *string {
	return &s
}
