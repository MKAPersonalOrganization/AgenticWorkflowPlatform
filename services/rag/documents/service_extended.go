package documents

// Extended service methods for additional Edgequake document endpoints.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"school-with-ai/domain"
)

// DocumentMetadataResponse wraps the JSON metadata from the API.
type DocumentMetadataResponse struct {
	DocumentID string      `json:"document_id"`
	Metadata   interface{} `json:"metadata"`
}

// ExportLineageResponse wraps exported lineage data.
type ExportLineageResponse struct {
	Data   interface{} `json:"data"`
	Format string      `json:"format"`
}

// CancelPdfResponse represents a successful PDF cancellation.
type CancelPdfResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// GetTrackStatus retrieves the status of a tracked upload batch.
func (s *Service) GetTrackStatus(ctx context.Context, tCtx *domain.TenantContext, trackID string) (*TrackStatusResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/track/%s", s.baseURL, trackID)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response TrackStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// RecoverStuck recovers documents stuck in processing.
func (s *Service) RecoverStuck(ctx context.Context, tCtx *domain.TenantContext) (*RecoverStuckResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/recover-stuck", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response RecoverStuckResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetDeletionImpact analyses the impact of deleting a document.
func (s *Service) GetDeletionImpact(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*DeletionImpactResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/%s/deletion-impact", s.baseURL, documentID)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response DeletionImpactResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// RetryFailedChunks retries failed chunks for a document.
func (s *Service) RetryFailedChunks(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*RetryChunksResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/%s/retry-chunks", s.baseURL, documentID)

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response RetryChunksResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// ListFailedChunks lists failed chunks for a document.
func (s *Service) ListFailedChunks(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*ListFailedChunksResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/%s/failed-chunks", s.baseURL, documentID)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ListFailedChunksResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetFullLineage retrieves full lineage for a document.
// The endpoint returns the document detail with lineage information.
func (s *Service) GetFullLineage(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*DocumentDetail, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/%s/lineage", s.baseURL, documentID)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response DocumentDetail
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetMetadata retrieves metadata for a document.
func (s *Service) GetMetadata(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*DocumentMetadataResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/%s/metadata", s.baseURL, documentID)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response DocumentMetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// ExportLineage exports lineage data for a document.
func (s *Service) ExportLineage(ctx context.Context, tCtx *domain.TenantContext, documentID string, format string) (*ExportLineageResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/%s/lineage/export", s.baseURL, documentID)
	if format != "" {
		urlStr += "?format=" + format
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ExportLineageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// CancelPdfProcessing cancels processing for a specific PDF.
func (s *Service) CancelPdfProcessing(ctx context.Context, tCtx *domain.TenantContext, pdfID string) (*CancelPdfResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/pdf/%s/cancel", s.baseURL, pdfID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response CancelPdfResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// UploadBatch uploads multiple documents in a batch.
func (s *Service) UploadBatch(ctx context.Context, tCtx *domain.TenantContext, files []io.Reader, fileNames []string) (*BatchUploadResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/documents/upload/batch", s.baseURL)

	var body io.Reader
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()
		for i, f := range files {
			name := "files"
			if i < len(fileNames) {
				name = fileNames[i]
			}
			part, err := writer.CreateFormFile("files", name)
			if err != nil {
				return
			}
			io.Copy(part, f)
		}
	}()
	body = pr

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response BatchUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}
