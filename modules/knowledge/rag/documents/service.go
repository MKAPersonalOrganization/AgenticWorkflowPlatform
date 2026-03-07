package documents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"school-with-ai/domain"
	"school-with-ai/modules/knowledge/rag/tasks"
	"time"
)

var (
	ErrUnauthorized = fmt.Errorf("documents: unauthorized")
	ErrForbidden    = fmt.Errorf("documents: access forbidden")
	ErrConflict     = fmt.Errorf("documents: conflict")
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
func (s *Service) List(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int, status string) (*ListDocumentsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents?page=%d&page_size=%d", s.baseURL, page, pageSize)
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

	var result ListDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// Get retrieves a single document by ID.
func (s *Service) Get(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*DocumentDetail, error) {
	url := fmt.Sprintf("%s/api/v1/documents/%s", s.baseURL, documentID)

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

	var result DocumentDetail
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
func (s *Service) ListPDFs(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int, status string, search string) (*ListPdfsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf?page=%d&page_size=%d", s.baseURL, page, pageSize)
	if status != "" {
		url += fmt.Sprintf("&status=%s", status)
	}
	if search != "" {
		url += fmt.Sprintf("&search=%s", search)
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

// DownloadPDF retrieves the raw PDF binary from Edgequake.
func (s *Service) DownloadPDF(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (io.ReadCloser, string, int64, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf/%s/download", s.baseURL, pdfID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", 0, fmt.Errorf("creating request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("executing request: %w", err)
	}

	if err := s.handleError(resp); err != nil {
		resp.Body.Close()
		return nil, "", 0, err
	}

	return resp.Body, resp.Header.Get("Content-Type"), resp.ContentLength, nil
}

// GetPipelineTasks retrieves the current tasks in the pipeline.
func (s *Service) GetPipelineTasks(ctx context.Context, tCtx *domain.TenantContext, page, pageSize int) (*tasks.TaskListResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tasks?page=%d&page_size=%d", s.baseURL, page, pageSize)

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

	var result tasks.TaskListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// Reprocess triggers reprocessing for a single document.
func (s *Service) Reprocess(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*ReprocessResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/reprocess", s.baseURL)
	reqBody := map[string]interface{}{
		"document_id":   documentID,
		"force":         true,
		"max_documents": 1,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var result ReprocessResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// ReprocessFailed triggers reprocessing for all failed documents.
func (s *Service) ReprocessFailed(ctx context.Context, tCtx *domain.TenantContext) (*ReprocessFailedResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/reprocess", s.baseURL)

	// Empty JSON body - the API distinguishes reprocess-all from single-doc reprocess by the absence of document_id
	body := bytes.NewReader([]byte("{}"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var result ReprocessFailedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// ScanDocuments triggers scanning of the input directory for new documents.
func (s *Service) ScanDocuments(ctx context.Context, tCtx *domain.TenantContext, scanReq ScanDocumentsRequest) (*ScanDocumentsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/scan", s.baseURL)

	body, _ := json.Marshal(scanReq)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var result ScanDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// DeleteAll deletes all documents from the workspace.
func (s *Service) DeleteAll(ctx context.Context, tCtx *domain.TenantContext) (*DeleteAllDocumentsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
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

	var result DeleteAllDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// RetryTask retries a failed task.
func (s *Service) RetryTask(ctx context.Context, tCtx *domain.TenantContext, taskID string) (*TaskRetryResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tasks/%s/retry", s.baseURL, taskID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
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

	var result TaskRetryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// RetryPdfProcessing retries a failed PDF processing.
func (s *Service) RetryPdfProcessing(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (*PdfRetryResponse, error) {
	url := fmt.Sprintf("%s/api/v1/documents/pdf/%s/retry", s.baseURL, pdfID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
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

	var result PdfRetryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// CancelTask cancels a task in the pipeline.
func (s *Service) CancelTask(ctx context.Context, tCtx *domain.TenantContext, taskID string) error {
	url := fmt.Sprintf("%s/api/v1/tasks/%s/cancel", s.baseURL, taskID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	return s.handleError(resp)
}

// Delete deletes a document from Edgequake.
func (s *Service) Delete(ctx context.Context, tCtx *domain.TenantContext, documentID string) error {
	url := fmt.Sprintf("%s/api/v1/documents/%s", s.baseURL, documentID)

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

	return s.handleError(resp)
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
	if resp.StatusCode < 400 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Restore for potential downstream use if needed, though unlikely here

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrForbidden
	}
	if resp.StatusCode == http.StatusConflict {
		return ErrConflict
	}
	if resp.StatusCode == http.StatusNotFound {
		errStr := string(body)
		if errStr == "" {
			errStr = "resource not found"
		}
		return fmt.Errorf("%s", errStr)
	}
	return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
}
