package edgequake

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

func TestService_UploadDocument(t *testing.T) {
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

	service := NewService(server.URL)
	content := strings.NewReader("hello world")

	resp, err := service.UploadDocument(context.Background(), tCtx, "test.txt", "text/plain", content)
	if err != nil {
		t.Fatalf("UploadDocument failed: %v", err)
	}

	if resp.DocumentID != "doc-123" {
		t.Errorf("Expected document ID doc-123, got %s", resp.DocumentID)
	}
}

func TestService_ListDocuments_Headers(t *testing.T) {
	tenantID := uuid.New()
	userID := "user-1"
	role := "member"

	tCtx := &domain.TenantContext{
		TenantID: &tenantID,
		UserID:   userID,
		Role:     role,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
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

	service := NewService(server.URL)
	_, err := service.ListDocuments(context.Background(), tCtx, 1, 10)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
}

func TestService_UploadAndListFlow(t *testing.T) {
	// 1. Setup Mock Server with state
	documents := []DocumentSummary{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/documents/upload" {
			// Handle Upload
			err := r.ParseMultipartForm(10 << 20)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			_, header, err := r.FormFile("file")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			newDoc := FileUploadResponse{
				DocumentID: uuid.New().String(),
				FileName:   header.Filename,
			}
			// Store in list as summary for list endpoint
			documents = append(documents, DocumentSummary{
				ID:       newDoc.DocumentID,
				FileName: &newDoc.FileName,
			})

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(newDoc)
			return
		}

		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/documents" {
			// Handle List
			json.NewEncoder(w).Encode(ListDocumentsResponse{
				Documents:  documents,
				Total:      len(documents),
				Page:       1,
				PageSize:   10,
				TotalPages: 1,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL)
	ctx := context.Background()
	tCtx := &domain.TenantContext{
		UserID: "test-user",
		Role:   "admin",
	}

	// 2. Initial list should be empty
	resp, err := service.ListDocuments(ctx, tCtx, 1, 10)
	if err != nil {
		t.Fatalf("Initial list failed: %v", err)
	}
	if len(resp.Documents) != 0 {
		t.Errorf("Expected 0 documents, got %d", len(resp.Documents))
	}

	// 3. Upload a document
	fileName := "report.pdf"
	content := strings.NewReader("pdf content")
	uploadResp, err := service.UploadDocument(ctx, tCtx, fileName, "application/pdf", content)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if uploadResp.FileName != fileName {
		t.Errorf("Expected filename %s, got %s", fileName, uploadResp.FileName)
	}

	// 4. List documents again - should contain the uploaded one
	resp, err = service.ListDocuments(ctx, tCtx, 1, 10)
	if err != nil {
		t.Fatalf("Second list failed: %v", err)
	}
	if len(resp.Documents) != 1 {
		t.Errorf("Expected 1 document, got %d", len(resp.Documents))
	}
	if resp.Documents[0].ID != uploadResp.DocumentID {
		t.Errorf("Expected document ID %s, got %s", uploadResp.DocumentID, resp.Documents[0].ID)
	}
	if *resp.Documents[0].FileName != fileName {
		t.Errorf("Expected filename %s, got %s", fileName, *resp.Documents[0].FileName)
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
		// Verify URL and Query Params
		if !strings.HasPrefix(r.URL.Path, "/api/v1/documents/pdf") {
			t.Errorf("Expected path starting with /api/v1/documents/pdf, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" {
			t.Errorf("Expected page 1, got %s", r.URL.Query().Get("page"))
		}
		if r.URL.Query().Get("page_size") != "20" {
			t.Errorf("Expected page_size 20, got %s", r.URL.Query().Get("page_size"))
		}
		if r.URL.Query().Get("status") != "processing" {
			t.Errorf("Expected status processing, got %s", r.URL.Query().Get("status"))
		}

		// Verify Headers
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ListPdfsResponse{
			Items: []PdfListItem{
				{
					PdfID:    "pdf-1",
					Filename: "test.pdf",
					Status:   "processing",
					FileSize: 1024,
				},
			},
			Pagination: PdfPaginationInfo{
				Page:       1,
				PageSize:   20,
				TotalCount: 1,
				TotalPages: 1,
			},
		})
	}))
	defer server.Close()

	service := NewService(server.URL)
	resp, err := service.ListPDFs(context.Background(), tCtx, 1, 20, "processing")
	if err != nil {
		t.Fatalf("ListPDFs failed: %v", err)
	}

	if len(resp.Items) != 1 {
		t.Errorf("Expected 1 PDF, got %d", len(resp.Items))
	}
	if resp.Items[0].PdfID != "pdf-1" {
		t.Errorf("Expected PDF ID pdf-1, got %s", resp.Items[0].PdfID)
	}
}

func TestService_DeletePDF(t *testing.T) {
	tenantID := uuid.New()
	workspaceID := uuid.New()
	pdfID := uuid.New().String()

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
		Role:        "admin",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL
		if r.URL.Path != fmt.Sprintf("/api/v1/documents/pdf/%s", pdfID) {
			t.Errorf("Expected path /api/v1/documents/pdf/%s, got %s", pdfID, r.URL.Path)
		}
		if r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE method, got %s", r.Method)
		}

		// Verify Headers
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service := NewService(server.URL)
	err := service.DeletePDF(context.Background(), tCtx, pdfID)
	if err != nil {
		t.Fatalf("DeletePDF failed: %v", err)
	}
}

func TestService_DeletePDF_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL)
	err := service.DeletePDF(context.Background(), nil, "invalid-id")
	if err == nil {
		t.Fatal("Expected error for non-existent PDF, got nil")
	}
	if err.Error() != "PDF not found" {
		t.Errorf("Expected 'PDF not found' error, got %v", err)
	}
}

func TestService_GetPDFProgress(t *testing.T) {
	trackID := fmt.Sprintf("upload-%s", uuid.New().String())
	pdfID := uuid.New().String()
	tenantID := uuid.New()
	workspaceID := uuid.New()

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
		Role:        "admin",
	}

	mockProgress := PdfUploadProgress{
		TrackID:           trackID,
		PdfID:             pdfID,
		OverallPercentage: 45.5,
		IsComplete:        false,
		Filename:          "test.pdf",
		Phases: []PhaseProgress{
			{
				Phase:      "upload",
				Status:     "complete",
				Percentage: 100,
			},
			{
				Phase:      "pdf_conversion",
				Status:     "active",
				Percentage: 50,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL
		if r.URL.Path != fmt.Sprintf("/api/v1/documents/pdf/progress/%s", trackID) {
			t.Errorf("Expected path /api/v1/documents/pdf/progress/%s, got %s", trackID, r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Verify Headers
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockProgress)
	}))
	defer server.Close()

	service := NewService(server.URL)
	progress, err := service.GetPDFProgress(context.Background(), tCtx, trackID)
	if err != nil {
		t.Fatalf("GetPDFProgress failed: %v", err)
	}

	if progress.TrackID != trackID {
		t.Errorf("Expected TrackID %s, got %s", trackID, progress.TrackID)
	}
	if progress.OverallPercentage != 45.5 {
		t.Errorf("Expected OverallPercentage 45.5, got %f", progress.OverallPercentage)
	}
	if len(progress.Phases) != 2 {
		t.Errorf("Expected 2 phases, got %d", len(progress.Phases))
	}
}

func TestService_GetPDFProgress_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL)
	_, err := service.GetPDFProgress(context.Background(), nil, "invalid-track-id")
	if err == nil {
		t.Fatal("Expected error for non-existent progress, got nil")
	}
	if err.Error() != "progress not found" {
		t.Errorf("Expected 'progress not found' error, got %v", err)
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
		Role:        "admin",
	}

	mockStatus := PdfStatusResponse{
		PdfID:  pdfID,
		Status: "completed",
		Metadata: PdfStatusMetadata{
			Filename:      "test.pdf",
			FileSizeBytes: 1024,
			VisionEnabled: false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL
		if r.URL.Path != fmt.Sprintf("/api/v1/documents/pdf/%s", pdfID) {
			t.Errorf("Expected path /api/v1/documents/pdf/%s, got %s", pdfID, r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Verify Headers
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockStatus)
	}))
	defer server.Close()

	service := NewService(server.URL)
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL)
	_, err := service.GetPDFStatus(context.Background(), nil, "invalid-pdf-id")
	if err == nil {
		t.Fatal("Expected error for non-existent PDF, got nil")
	}
	if err.Error() != "PDF not found" {
		t.Errorf("Expected 'PDF not found' error, got %v", err)
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
		// Verify URL
		if r.URL.Path != fmt.Sprintf("/api/v1/documents/pdf/%s/content", pdfID) {
			t.Errorf("Expected path /api/v1/documents/pdf/%s/content, got %s", pdfID, r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		// Verify Headers
		if r.Header.Get("X-Tenant-ID") != tenantID.String() {
			t.Errorf("Expected X-Tenant-ID %s, got %s", tenantID.String(), r.Header.Get("X-Tenant-ID"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockContent)
	}))
	defer server.Close()

	service := NewService(server.URL)
	content, err := service.GetPDFContent(context.Background(), tCtx, pdfID)
	if err != nil {
		t.Fatalf("GetPDFContent failed: %v", err)
	}

	if content.PdfID != pdfID {
		t.Errorf("Expected PdfID %s, got %s", pdfID, content.PdfID)
	}
	if content.MarkdownContent == nil || *content.MarkdownContent != contentStr {
		t.Errorf("Expected MarkdownContent %s, got %v", contentStr, content.MarkdownContent)
	}
	if !content.IsProcessed {
		t.Error("Expected IsProcessed true, got false")
	}
}

func TestService_GetPDFContent_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(server.URL)
	_, err := service.GetPDFContent(context.Background(), nil, "invalid-pdf-id")
	if err == nil {
		t.Fatal("Expected error for non-existent PDF, got nil")
	}
	if err.Error() != "PDF not found" {
		t.Errorf("Expected 'PDF not found' error, got %v", err)
	}
}
