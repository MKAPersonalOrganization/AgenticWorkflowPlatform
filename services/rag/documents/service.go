package documents

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
	ErrUnauthorized = fmt.Errorf("documents: unauthorized")
	ErrForbidden    = fmt.Errorf("documents: access forbidden")
)

// Service provides access to the Edgequake Document ingestion API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Documents service.
func NewService(baseURL string, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	return &Service{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// List retrieves a paginated list of documents from Edgequake.
func (s *Service) List(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int) (*ListDocumentsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents?page=%d&page_size=%d", s.baseURL, page, pageSize)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var result ListDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// Upload uploads a document to Edgequake.
func (s *Service) Upload(ctx context.Context, tCtx *domain.TenantContext, fileName string, contentType string, content io.Reader) (*FileUploadResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/upload", s.baseURL)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	if _, err = io.Copy(fw, content); err != nil {
		return nil, fmt.Errorf("copying content: %w", err)
	}

	if err = w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
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
		url += fmt.Sprintf("&status=%s", status)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
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

	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	if _, err = io.Copy(fw, content); err != nil {
		return nil, fmt.Errorf("copying content: %w", err)
	}

	if err = w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
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

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		if err.Error() == "PDF not found" {
			return nil // Idempotent delete
		}
		return err
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

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
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

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
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

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var result PdfContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

func (s *Service) injectHeaders(req *http.Request, tCtx *domain.TenantContext) {
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
}

func (s *Service) handleError(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("PDF not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
