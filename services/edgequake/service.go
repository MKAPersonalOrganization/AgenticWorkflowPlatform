package edgequake

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"school-with-ai/domain"
)

var (
	ErrEdgequakeUnauthorized = fmt.Errorf("edgequake: unauthorized")
	ErrEdgequakeForbidden    = fmt.Errorf("edgequake: access forbidden")
)

// Service provides access to the Edgequake REST API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Edgequake service.
func NewService(baseURL string) *Service {
	return &Service{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ListDocuments retrieves a paginated list of documents from Edgequake.
func (s *Service) ListDocuments(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int) (*ListDocumentsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents?page=%d&page_size=%d", s.baseURL, page, pageSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrEdgequakeForbidden
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result ListDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// UploadDocument uploads a document to Edgequake.
func (s *Service) UploadDocument(ctx context.Context, tCtx *domain.TenantContext, fileName string, contentType string, content io.Reader) (*FileUploadResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/upload", s.baseURL)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Create a form file field
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	// Copy the content to the form file field
	if _, err = io.Copy(fw, content); err != nil {
		return nil, fmt.Errorf("copying content: %w", err)
	}

	// Close the multipart writer to finalize the body
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set Content-Type from the multipart writer
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrEdgequakeForbidden
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result FileUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// ListPDFs retrieves a paginated list of PDFs from Edgequake.
func (s *Service) ListPDFs(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int, status string) (*ListPdfsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf?page=%d&page_size=%d", s.baseURL, page, pageSize)
	if status != "" {
		url = fmt.Sprintf("%s&status=%s", url, status)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrEdgequakeForbidden
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result ListPdfsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// UploadPDF uploads a PDF document to Edgequake.
func (s *Service) UploadPDF(ctx context.Context, tCtx *domain.TenantContext, fileName string, content io.Reader) (*PdfUploadResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf", s.baseURL)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Create a form file field
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	// Copy the content to the form file field
	if _, err = io.Copy(fw, content); err != nil {
		return nil, fmt.Errorf("copying content: %w", err)
	}

	// Close the multipart writer to finalize the body
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set Content-Type from the multipart writer
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrEdgequakeForbidden
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result PdfUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// DeletePDF deletes a PDF document from Edgequake.
func (s *Service) DeletePDF(ctx context.Context, tCtx *domain.TenantContext, pdfID string) error {
	url := fmt.Sprintf("%s/api/v1/documents/pdf/%s", s.baseURL, pdfID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrEdgequakeForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("PDF not found")
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetPDFProgress retrieves the progress of a PDF upload by track ID.
func (s *Service) GetPDFProgress(ctx context.Context, tCtx *domain.TenantContext, trackID string) (*PdfUploadProgress, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf/progress/%s", s.baseURL, trackID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrEdgequakeForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("progress not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result PdfUploadProgress
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// GetPDFStatus retrieves the status of a specific PDF.
func (s *Service) GetPDFStatus(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (*PdfStatusResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf/%s", s.baseURL, pdfID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrEdgequakeForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("PDF not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result PdfStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// GetPDFContent retrieves the content of a specific PDF.
func (s *Service) GetPDFContent(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (*PdfContentResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf/%s/content", s.baseURL, pdfID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrEdgequakeUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrEdgequakeForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("PDF not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result PdfContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}
