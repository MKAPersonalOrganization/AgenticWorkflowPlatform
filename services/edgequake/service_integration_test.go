//go:build integration

package edgequake

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"school-with-ai/domain"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// To run these tests:
// EDGEQUAKE_BASE_URL=http://localhost:8081 go test -v -tags=integration ./services/edgequake/...

func TestService_Integration_UploadAndList(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL)
	ctx := context.Background()

	// Use dev seed IDs or provided ones
	tenantIDStr := os.Getenv("TEST_TENANT_ID")
	workspaceIDStr := os.Getenv("TEST_WORKSPACE_ID")
	userID := os.Getenv("TEST_USER_ID")
	role := os.Getenv("TEST_USER_ROLE")

	// Fallback to dev seeds if not provided
	if tenantIDStr == "" {
		tenantIDStr = "00000000-0000-0000-0000-000000000001"
	}
	if workspaceIDStr == "" {
		workspaceIDStr = "00000000-0000-0000-0000-000000000003" // Default workspace UUID
	}
	if userID == "" {
		userID = "00000000-0000-0000-0000-000000000001"
	}
	if role == "" {
		role = "admin"
	}

	id1, _ := uuid.Parse(tenantIDStr)
	tenantID := &id1

	var workspaceID *uuid.UUID
	if workspaceIDStr != "" {
		id, _ := uuid.Parse(workspaceIDStr)
		workspaceID = &id
	}

	tCtx := &domain.TenantContext{
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
	}

	fmt.Printf("Running integration test against %s\n", baseURL)
	fmt.Printf("Context: User=%s, Role=%s\n", userID, role)

	// 1. List current documents
	initialResp, err := service.ListDocuments(ctx, tCtx, 1, 10)
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}
	initialCount := len(initialResp.Documents)
	fmt.Printf("Initial document count: %d\n", initialCount)

	// 2. Upload a new document
	fileName := fmt.Sprintf("integration-test-%d.txt", uuid.New().ID())
	content := strings.NewReader("EdgeQuake Corp is developing a new agentic coding assistant called Antigravity. The project is led by the Google DeepMind team and aims to revolutionize software development through advanced AI automation and pair programming capabilities.")

	uploadResp, err := service.UploadDocument(ctx, tCtx, fileName, "text/plain", content)
	if err != nil {
		t.Fatalf("Failed to upload document: %v", err)
	}
	fmt.Printf("Uploaded document ID: %s\n", uploadResp.DocumentID)

	// 3. List documents again and verify the new one is there
	finalResp, err := service.ListDocuments(ctx, tCtx, 1, 100) // Increase page size to ensure we find it
	if err != nil {
		t.Fatalf("Failed to list documents after upload: %v", err)
	}

	found := false
	for _, doc := range finalResp.Documents {
		if doc.ID == uploadResp.DocumentID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Uploaded document %s not found in listing", uploadResp.DocumentID)
	}

	if len(finalResp.Documents) <= initialCount && initialResp.TotalPages == 1 {
		// This check might be flakey if others are uploading, but general check
		t.Logf("Warning: Final count %d is not greater than initial count %d (might be pagination or concurrent edits)", len(finalResp.Documents), initialCount)
	}

	fmt.Printf("Integration test passed. Final document count: %d\n", len(finalResp.Documents))
}

func TestService_Integration_ListPDFs(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL)
	ctx := context.Background()

	// Use dev seed IDs for tenant context
	tenantIDStr := "00000000-0000-0000-0000-000000000001"
	workspaceIDStr := "00000000-0000-0000-0000-000000000003"
	userID := "00000000-0000-0000-0000-000000000001"
	role := "admin"

	id1, _ := uuid.Parse(tenantIDStr)
	tenantID := &id1
	id2, _ := uuid.Parse(workspaceIDStr)
	workspaceID := &id2

	tCtx := &domain.TenantContext{
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
	}

	fmt.Printf("Running PDF listing integration test against %s\n", baseURL)

	resp, err := service.ListPDFs(ctx, tCtx, 1, 10, "")
	if err != nil {
		t.Fatalf("ListPDFs failed: %v", err)
	}

	fmt.Printf("Found %d PDFs in workspace\n", resp.Pagination.TotalCount)
	for _, pdf := range resp.Items {
		fmt.Printf("- PDF: %s (ID: %s, Status: %s)\n", pdf.Filename, pdf.PdfID, pdf.Status)
	}

	// We don't necessarily expect PDFs to exist in a fresh dev env,
	// but the request should succeed.
	if resp.Pagination.Page != 1 {
		t.Errorf("Expected page 1, got %d", resp.Pagination.Page)
	}

	// 1. Upload a dummy PDF
	pdfName := fmt.Sprintf("integration-test-pdf-%d.pdf", uuid.New().ID())
	// Minimal valid PDF header to pass basic validation if any
	pdfContent := strings.NewReader("%PDF-1.4\n%\n1 0 obj\n<<\n/Type /Catalog\n/Pages 2 0 R\n>>\nendobj\n2 0 obj\n<<\n/Type /Pages\n/Kids [3 0 R]\n/Count 1\n>>\nendobj\n3 0 obj\n<<\n/Type /Page\n/Parent 2 0 R\n/MediaBox [0 0 612 792]\n/Resources << >>\n>>\nendobj\nxref\n0 4\n0000000000 65535 f \n0000000010 00000 n \n0000000060 00000 n \n0000000111 00000 n \ntrailer\n<<\n/Size 4\n/Root 1 0 R\n>>\nstartxref\n190\n%%EOF")

	uploadResp, err := service.UploadPDF(ctx, tCtx, pdfName, pdfContent)
	if err != nil {
		t.Fatalf("Failed to upload PDF: %v", err)
	}
	fmt.Printf("Uploaded PDF ID: %s\n", uploadResp.PdfID)

	// 2. List PDFs and verify the new one is there
	// Poll a few times as indexing might take a moment, though listing usually shows 'processing' immediately
	finalResp, err := service.ListPDFs(ctx, tCtx, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list PDFs after upload: %v", err)
	}

	found := false
	for _, pdf := range finalResp.Items {
		if pdf.PdfID == uploadResp.PdfID {
			found = true
			fmt.Printf("Found uploaded PDF: %s (Status: %s)\n", pdf.Filename, pdf.Status)
			break
		}
	}

	if !found {
		t.Errorf("Uploaded PDF %s not found in listing", uploadResp.PdfID)
	}

	// 3. Delete the PDF
	if err := service.DeletePDF(ctx, tCtx, uploadResp.PdfID); err != nil {
		t.Fatalf("Failed to delete PDF: %v", err)
	}
	fmt.Printf("Deleted PDF ID: %s\n", uploadResp.PdfID)

	// 4. Verify deletion
	finalResp2, err := service.ListPDFs(ctx, tCtx, 1, 100, "")
	if err != nil {
		t.Fatalf("Failed to list PDFs after deletion: %v", err)
	}

	foundAfterDelete := false
	for _, pdf := range finalResp2.Items {
		if pdf.PdfID == uploadResp.PdfID {
			foundAfterDelete = true
			break
		}
	}

	if foundAfterDelete {
		t.Errorf("PDF %s still found in listing after deletion", uploadResp.PdfID)
	}
}

func TestService_Integration_GetPDFProgress(t *testing.T) {
	if os.Getenv("EDGEQUAKE_BASE_URL") == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	fmt.Printf("Running PDF progress integration test against %s\n", baseURL)

	service := NewService(baseURL)

	// Create a dummy PDF file content
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> /Contents 4 0 R >>\nendobj\n4 0 obj\n<< /Length 21 >>\nstream\nBT /F1 12 Tf 100 700 Td (Hello World) Tj ET\nendstream\nendobj\nxref\n0 5\n0000000000 65535 f \n0000000010 00000 n \n0000000060 00000 n \n0000000117 00000 n \n0000000224 00000 n \ntrailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n294\n%%EOF\n")
	fileName := fmt.Sprintf("integration-test-progress-%d.pdf", time.Now().Unix())

	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
		Role:        "admin",
	}

	// 1. Upload PDF
	uploadResp, err := service.UploadPDF(ctx, tCtx, fileName, bytes.NewReader(pdfContent))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key constraint") {
			t.Skipf("Skipping integration test due to missing tenant/workspace setup: %v", err)
		}
		t.Fatalf("Failed to upload PDF: %v", err)
	}
	fmt.Printf("Uploaded PDF ID: %s, Track ID: %s\n", uploadResp.PdfID, uploadResp.TaskID)

	// 2. Check Progress
	// We might need to retry a few times as the task might take a moment to be created
	var progress *PdfUploadProgress
	for i := 0; i < 5; i++ {
		progress, err = service.GetPDFProgress(ctx, tCtx, uploadResp.TaskID)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to get PDF progress: %v", err)
	}

	fmt.Printf("Progress: ID=%s, Filename=%s, Overall=%.2f%%\n", progress.TrackID, progress.Filename, progress.OverallPercentage)
	for _, p := range progress.Phases {
		fmt.Printf(" - Phase: %-15s Status: %-10s Percent: %.1f%%\n", p.Phase, p.Status, p.Percentage)
	}

	if progress.TrackID != uploadResp.TaskID {
		t.Errorf("Expected TrackID %s, got %s", uploadResp.TaskID, progress.TrackID)
	}

	if len(progress.Phases) == 0 {
		t.Error("Expected at least one phase in progress")
	}

	// Clean up (optional, but good practice)
	_ = service.DeletePDF(ctx, tCtx, uploadResp.PdfID)
}

func TestService_Integration_GetPDFStatus(t *testing.T) {
	if os.Getenv("EDGEQUAKE_BASE_URL") == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	fmt.Printf("Running PDF status integration test against %s\n", baseURL)

	service := NewService(baseURL)

	// Create a dummy PDF file content
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << >> /Contents 4 0 R >>\nendobj\n4 0 obj\n<< /Length 21 >>\nstream\nBT /F1 12 Tf 100 700 Td (Hello World) Tj ET\nendstream\nendobj\nxref\n0 5\n0000000000 65535 f \n0000000010 00000 n \n0000000060 00000 n \n0000000117 00000 n \n0000000224 00000 n \ntrailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n294\n%%EOF\n")
	fileName := fmt.Sprintf("integration-test-status-%d.pdf", time.Now().Unix())

	ctx := context.Background()
	// Use valid tenant/workspace IDs found previously
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
		Role:        "admin",
	}

	// 1. Upload PDF
	uploadResp, err := service.UploadPDF(ctx, tCtx, fileName, bytes.NewReader(pdfContent))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key constraint") {
			t.Skipf("Skipping integration test due to missing tenant/workspace setup: %v", err)
		}
		t.Fatalf("Failed to upload PDF: %v", err)
	}
	fmt.Printf("Uploaded PDF ID: %s\n", uploadResp.PdfID)

	// 2. Check Status
	// Poll a few times as indexing might take a moment
	var status *PdfStatusResponse
	for i := 0; i < 5; i++ {
		status, err = service.GetPDFStatus(ctx, tCtx, uploadResp.PdfID)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to get PDF status: %v", err)
	}

	fmt.Printf("Status: ID=%s, Filename=%s, State=%s\n", status.PdfID, status.Metadata.Filename, status.Status)

	if status.PdfID != uploadResp.PdfID {
		t.Errorf("Expected PdfID %s, got %s", uploadResp.PdfID, status.PdfID)
	}
	if status.Metadata.Filename != fileName {
		t.Errorf("Expected Filename %s, got %s", fileName, status.Metadata.Filename)
	}

	// Clean up
	_ = service.DeletePDF(ctx, tCtx, uploadResp.PdfID)
}

func TestService_Integration_GetPDFContent(t *testing.T) {
	if os.Getenv("EDGEQUAKE_BASE_URL") == "" {
		t.Skip("Skipping integration test: EDGEQUAKE_BASE_URL not set")
	}

	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	fmt.Printf("Running PDF content integration test against %s\n", baseURL)

	service := NewService(baseURL)

	// Create a valid PDF file content with text "Hello World"
	// This is a minimal valid PDF
	pdfContent := []byte("%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n" +
		"4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n" +
		"5 0 obj\n<< /Length 22 >>\nstream\nBT /F1 24 Tf 100 700 Td (Hello World) Tj ET\nendstream\nendobj\n" +
		"xref\n0 6\n0000000000 65535 f \n0000000010 00000 n \n0000000060 00000 n \n0000000117 00000 n \n0000000256 00000 n \n0000000315 00000 n \n" +
		"trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n386\n%%EOF\n")
	fileName := fmt.Sprintf("integration-test-content-%d.pdf", time.Now().Unix())

	ctx := context.Background()
	// Use valid tenant/workspace IDs found previously
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      "test-user",
		Role:        "admin",
	}

	// 1. Upload PDF
	uploadResp, err := service.UploadPDF(ctx, tCtx, fileName, bytes.NewReader(pdfContent))
	if err != nil {
		if strings.Contains(err.Error(), "foreign key constraint") {
			t.Skipf("Skipping integration test due to missing tenant/workspace setup: %v", err)
		}
		t.Fatalf("Failed to upload PDF: %v", err)
	}
	fmt.Printf("Uploaded PDF ID: %s\n", uploadResp.PdfID)

	// 2. Wait for Processing to Complete
	// Poll status until completed or timeout (max 30 seconds)
	fmt.Println("Waiting for PDF processing to complete...")
	for i := 0; i < 30; i++ {
		status, err := service.GetPDFStatus(ctx, tCtx, uploadResp.PdfID)
		if err == nil && status.Status == "completed" {
			fmt.Println("PDF processing completed!")
			break
		}
		if err == nil && status.Status == "failed" {
			t.Fatalf("PDF processing failed: %v", status.Errors)
		}
		time.Sleep(1 * time.Second)
	}

	// 3. Check Content Metadata
	content, err := service.GetPDFContent(ctx, tCtx, uploadResp.PdfID)
	if err != nil {
		t.Fatalf("Failed to get PDF content: %v", err)
	}

	fmt.Printf("Content: ID=%s, Filename=%s, Processed=%v\n", content.PdfID, content.Filename, content.IsProcessed)
	if content.MarkdownContent != nil {
		fmt.Printf("Extracted Markdown Length: %d\n", len(*content.MarkdownContent))
		fmt.Printf("Preview: %s\n", (*content.MarkdownContent)[:min(len(*content.MarkdownContent), 100)])

		if !strings.Contains(*content.MarkdownContent, "Hello World") {
			t.Errorf("Expected content to contain 'Hello World', got: %s", *content.MarkdownContent)
		}
	} else {
		t.Error("Markdown content is nil (extraction failed)")
	}

	if content.PdfID != uploadResp.PdfID {
		t.Errorf("Expected PdfID %s, got %s", uploadResp.PdfID, content.PdfID)
	}
	if content.Filename != fileName {
		t.Errorf("Expected Filename %s, got %s", fileName, content.Filename)
	}

	// Clean up
	_ = service.DeletePDF(ctx, tCtx, uploadResp.PdfID)
}
