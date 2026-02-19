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

// TrackStatusResponse represents the status of a tracked upload.
type TrackStatusResponse struct {
	TrackID string `json:"track_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// RecoverStuckResponse represents the result of recovering stuck documents.
type RecoverStuckResponse struct {
	Recovered int    `json:"recovered"`
	Message   string `json:"message"`
}

// DeletionImpactResponse represents the impact of deleting a document.
type DeletionImpactResponse struct {
	DocumentID   string `json:"document_id"`
	ChunksCount  int    `json:"chunks_count"`
	EntitiesLost int    `json:"entities_lost"`
	Message      string `json:"message,omitempty"`
}

// FailedChunkItem represents a single failed chunk.
type FailedChunkItem struct {
	ChunkID string `json:"chunk_id"`
	Error   string `json:"error"`
}

// FailedChunksResponse represents a list of failed chunks.
type FailedChunksResponse struct {
	Chunks []FailedChunkItem `json:"chunks"`
	Total  int               `json:"total"`
}

// RetryChunksResponse represents the result of retrying failed chunks.
type RetryChunksResponse struct {
	Retried int    `json:"retried"`
	Message string `json:"message"`
}

// DocumentLineageResponse represents full lineage for a document.
type DocumentLineageResponse struct {
	DocumentID string      `json:"document_id"`
	Lineage    interface{} `json:"lineage"`
}

// DocumentMetadataResponse represents metadata for a document.
type DocumentMetadataResponse struct {
	DocumentID string      `json:"document_id"`
	Metadata   interface{} `json:"metadata"`
}

// ExportLineageResponse represents exported lineage data.
type ExportLineageResponse struct {
	Data   interface{} `json:"data"`
	Format string      `json:"format"`
}

// CancelPdfResponse represents a successful PDF cancellation.
type CancelPdfResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// BatchUploadResponse represents a batch upload result.
type BatchUploadResponse struct {
	Uploaded int    `json:"uploaded"`
	TrackID  string `json:"track_id,omitempty"`
	Message  string `json:"message"`
}

// GetTrackStatus retrieves the status of a tracked upload.
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

// GetDeletionImpact analyzes the impact of deleting a document.
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
func (s *Service) ListFailedChunks(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*FailedChunksResponse, error) {
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

	var response FailedChunksResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetFullLineage retrieves full lineage for a document.
func (s *Service) GetFullLineage(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*DocumentLineageResponse, error) {
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

	var response DocumentLineageResponse
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
