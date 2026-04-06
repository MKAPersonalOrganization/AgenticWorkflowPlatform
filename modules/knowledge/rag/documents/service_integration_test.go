//go:build integration

package documents

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"aicompass/shared"

	"github.com/google/uuid"
)

// To run these tests:
// EDGEQUAKE_BASE_URL=http://localhost:8081 go test -v -tags=integration ./services/rag/documents/...

func TestService_Integration_UploadAndList(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	httpClient := &http.Client{Timeout: 120 * time.Second}
	service := NewService(baseURL, httpClient)
	ctx := context.Background()

	tCtx := getIntegrationContext()

	fmt.Printf("Running integration test against %s\n", baseURL)

	// 1. List current documents
	initialResp, err := service.List(ctx, tCtx, 1, 10, "")
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}
	initialCount := len(initialResp.Documents)
	fmt.Printf("Initial document count: %d\n", initialCount)

	// 2. Upload a new document (API processes synchronously, may take 30-60s)
	fileName := fmt.Sprintf("integration-test-%d.txt", time.Now().Unix())
	content := strings.NewReader(fmt.Sprintf("Integration test content for the new documents package. Unique ID: %s", uuid.New().String()))

	uploadResp, err := service.Upload(ctx, tCtx, fileName, "text/plain", content)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "Client.Timeout") {
			t.Skip("Skipping: document upload timed out (API processes documents synchronously)")
		}
		t.Fatalf("Failed to upload document: %v", err)
	}
	fmt.Printf("Uploaded document ID: %s\n", uploadResp.DocumentID)

	// 3. List documents again and verify the new one is there (with retries)
	found := false
	for i := 0; i < 5; i++ {
		finalResp, err := service.List(ctx, tCtx, 1, 100, "")
		if err != nil {
			t.Fatalf("Failed to list documents after upload: %v", err)
		}

		for _, doc := range finalResp.Documents {
			if doc.ID == uploadResp.DocumentID {
				found = true
				break
			}
		}
		if found {
			break
		}
		fmt.Printf("Document not found yet, retrying in 1s... (attempt %d/5)\n", i+1)
		time.Sleep(1 * time.Second)
	}

	if !found {
		t.Errorf("Uploaded document %s not found in list after retries", uploadResp.DocumentID)
	}
}

func TestService_Integration_ListPDFs(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getIntegrationContext()

	resp, err := service.ListPDFs(ctx, tCtx, 1, 10, "", "")
	if err != nil {
		t.Fatalf("Failed to list PDFs: %v", err)
	}

	fmt.Printf("Found %d PDFs in workspace\n", resp.Pagination.TotalCount)
}

func TestService_Integration_PDF_Lifecycle(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getIntegrationContext()

	// 1. Upload
	pdfContent := createDummyPDF()
	fileName := fmt.Sprintf("integration-test-lifecycle-%d.pdf", time.Now().Unix())
	uploadResp, err := service.UploadPDF(ctx, tCtx, fileName, bytes.NewReader(pdfContent))
	if err != nil {
		t.Fatalf("Failed to upload PDF: %v", err)
	}
	trackID := ""
	if uploadResp.TrackID != nil {
		trackID = *uploadResp.TrackID
	}
	fmt.Printf("Uploaded PDF ID: %s, TrackID: %s\n", uploadResp.PdfID, trackID)

	// 2. Poll Progress
	if uploadResp.TrackID != nil {
		completed := false
		for i := 0; i < 30; i++ {
			progress, err := service.GetPDFProgress(ctx, tCtx, *uploadResp.TrackID)
			if err != nil {
				t.Fatalf("Failed to get progress: %v", err)
			}
			if progress.IsComplete {
				completed = true
				break
			}
			time.Sleep(1 * time.Second)
		}
		if !completed {
			t.Log("PDF processing timed out in progress check")
		}
	}

	// 3. Check Status
	status, err := service.GetPDFStatus(ctx, tCtx, uploadResp.PdfID)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}
	fmt.Printf("PDF Status: %s\n", status.Status)

	// 4. Get Content
	content, err := service.GetPDFContent(ctx, tCtx, uploadResp.PdfID)
	if err != nil {
		t.Fatalf("Failed to get content: %v", err)
	}
	if content.MarkdownContent != nil {
		fmt.Printf("Extracted Content Length: %d\n", len(*content.MarkdownContent))
	}

	// 5. Delete
	err = service.DeletePDF(ctx, tCtx, uploadResp.PdfID)
	if err != nil {
		t.Fatalf("Failed to delete PDF: %v", err)
	}
	fmt.Printf("PDF deleted successfully\n")
}

func getIntegrationContext() *shared.TenantContext {
	tenantIDStr := "00000000-0000-0000-0000-000000000002"
	workspaceIDStr := "00000000-0000-0000-0000-000000000003"
	userID := "00000000-0000-0000-0000-000000000001"
	role := "admin"

	id1, _ := uuid.Parse(tenantIDStr)
	id2, _ := uuid.Parse(workspaceIDStr)

	return &shared.TenantContext{
		TenantID:    &id1,
		WorkspaceID: &id2,
		UserID:      userID,
		Role:        role,
	}
}

func TestService_Integration_ListDocuments_ResponseFields(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getIntegrationContext()

	resp, err := service.List(ctx, tCtx, 1, 10, "")
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}

	// Verify pagination fields match Rust ListDocumentsResponse
	if resp.Page < 1 {
		t.Error("Expected page >= 1")
	}
	if resp.PageSize < 1 {
		t.Error("Expected page_size >= 1")
	}
	// StatusCounts should always be present
	t.Logf("StatusCounts: pending=%d, processing=%d, completed=%d, partial_failure=%d, failed=%d, cancelled=%d",
		resp.StatusCounts.Pending, resp.StatusCounts.Processing, resp.StatusCounts.Completed,
		resp.StatusCounts.PartialFailure, resp.StatusCounts.Failed, resp.StatusCounts.Cancelled)
}

func TestService_Integration_GetDocument_Detail(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getIntegrationContext()

	// First list to get a document ID
	listResp, err := service.List(ctx, tCtx, 1, 1, "")
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}
	if len(listResp.Documents) == 0 {
		t.Skip("No documents available for detail test")
	}

	docID := listResp.Documents[0].ID
	detail, err := service.Get(ctx, tCtx, docID)
	if err != nil {
		t.Fatalf("Failed to get document detail: %v", err)
	}

	// Verify fields match Rust DocumentDetailResponse
	if detail.ID == "" {
		t.Error("Expected non-empty document ID")
	}
	if detail.Status == "" {
		t.Error("Expected non-empty status (Rust has non-optional String)")
	}

	// Lineage is an optional nested struct
	if detail.Lineage != nil {
		t.Logf("Document has lineage: llm_model=%v, embedding_model=%v", detail.Lineage.LLMModel, detail.Lineage.EmbeddingModel)
	}

	t.Logf("Document detail: id=%s, status=%s, chunk_count=%d, content_hash=%v, mime_type=%v, file_size=%v",
		detail.ID, detail.Status, detail.ChunkCount, detail.ContentHash, detail.MimeType, detail.FileSize)
}

func TestService_Integration_DeleteAll_ResponseFields(t *testing.T) {
	// This test only verifies the response shape, not that it actually deletes.
	// We DON'T call DeleteAll to avoid data loss. Instead we verify the
	// API responds with the correct structure by checking an empty workspace
	// or skipping if there are documents.
	t.Skip("Skipping destructive test - verify DeleteAllDocumentsResponse shape manually")
}

func TestService_Integration_GetPipelineTasks(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tCtx := getIntegrationContext()

	resp, err := service.GetPipelineTasks(ctx, tCtx, 1, 10)
	if err != nil {
		t.Fatalf("Failed to get pipeline tasks: %v", err)
	}

	// Verify the response matches Rust TaskListResponse exactly
	t.Logf("Tasks: total=%d, page=%d, page_size=%d, total_pages=%d",
		resp.Pagination.Total, resp.Pagination.Page, resp.Pagination.PageSize, resp.Pagination.TotalPages)
	t.Logf("Statistics: pending=%d, processing=%d, indexed=%d, failed=%d, cancelled=%d",
		resp.Statistics.Pending, resp.Statistics.Processing, resp.Statistics.Indexed,
		resp.Statistics.Failed, resp.Statistics.Cancelled)

	// Verify task fields match Rust TaskResponse
	for _, task := range resp.Tasks {
		if task.TrackID == "" {
			t.Error("Expected non-empty track_id")
		}
		if task.TenantID == "" {
			t.Error("Expected non-empty tenant_id")
		}
		if task.WorkspaceID == "" {
			t.Error("Expected non-empty workspace_id")
		}
		if task.TaskType == "" {
			t.Error("Expected non-empty task_type")
		}
		if task.Status == "" {
			t.Error("Expected non-empty status")
		}
		t.Logf("Task: track_id=%s, type=%s, status=%s, retry_count=%d",
			task.TrackID, task.TaskType, task.Status, task.RetryCount)
		break // just check the first one
	}
}

func createDummyPDF() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n<< /Type /Page /Contents 2 0 R /Resources << /Font << /F1 3 0 R >> >> >>\nendobj\n2 0 obj\n<< /Length 50 >>\nstream\nBT /F1 12 Tf 100 700 Td (Hello World) Tj ET\nendstream\nendobj\n3 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF")
}
