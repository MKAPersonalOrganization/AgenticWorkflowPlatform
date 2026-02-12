package lightrag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// DefaultURL is the default URL for the LightRAG API.
const DefaultURL = "http://localhost:9621"

// Query modes
const (
	ModeLocal  = "local"
	ModeGlobal = "global"
	ModeHybrid = "hybrid"
	ModeNaive  = "naive"
	ModeMix    = "mix"
	ModeBypass = "bypass"
)

// Client represents a client for the LightRAG API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	tracer     trace.Tracer
}

// NewClient creates a new LightRAG API client.
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: httpClient,
		tracer:     otel.Tracer("foundation/lightrag"),
	}
}

// QueryRequest represents a request to the query endpoint.
type QueryRequest struct {
	Query               string `json:"query"`
	Mode                string `json:"mode,omitempty"`
	Stream              bool   `json:"stream,omitempty"`
	OnlyNeedContext     bool   `json:"only_need_context,omitempty"`
	OnlyNeedPrompt      bool   `json:"only_need_prompt,omitempty"`
	IncludeReferences   bool   `json:"include_references,omitempty"`
	IncludeChunkContent bool   `json:"include_chunk_content,omitempty"`
	EnableRerank        *bool  `json:"enable_rerank,omitempty"`
	ResponseType        string `json:"response_type,omitempty"`
}

// Reference representation in the response.
type Reference struct {
	ReferenceID string   `json:"reference_id"`
	FilePath    string   `json:"file_path"`
	Content     []string `json:"content,omitempty"`
}

// DocStatus represents the processing status of a document.
type DocStatus string

const (
	StatusPending      DocStatus = "PENDING"
	StatusProcessing   DocStatus = "PROCESSING"
	StatusPreprocessed DocStatus = "PREPROCESSED"
	StatusProcessed    DocStatus = "PROCESSED"
	StatusFailed       DocStatus = "FAILED"
)

// DocStatusResponse represents the status of a single document.
type DocStatusResponse struct {
	ID             string                 `json:"id"`
	ContentSummary string                 `json:"content_summary"`
	ContentLength  int                    `json:"content_length"`
	Status         DocStatus              `json:"status"`
	CreatedAt      string                 `json:"created_at"`
	UpdatedAt      string                 `json:"updated_at"`
	TrackID        *string                `json:"track_id,omitempty"`
	ChunksCount    *int                   `json:"chunks_count,omitempty"`
	ErrorMsg       *string                `json:"error_msg,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	FilePath       string                 `json:"file_path"`
}

// DocsStatusesResponse represents the response from the /documents endpoint.
type DocsStatusesResponse struct {
	Statuses map[DocStatus][]DocStatusResponse `json:"statuses"`
}

// DocumentsRequest represents a request for paginated documents.
type DocumentsRequest struct {
	StatusFilter  *DocStatus `json:"status_filter,omitempty"`
	Page          int        `json:"page"`
	PageSize      int        `json:"page_size"`
	SortField     string     `json:"sort_field,omitempty"`     // created_at, updated_at, id, file_path
	SortDirection string     `json:"sort_direction,omitempty"` // asc, desc
}

// PaginationInfo represents pagination metadata.
type PaginationInfo struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalCount int  `json:"total_count"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// PaginatedDocsResponse represents the response from the /documents/paginated endpoint.
type PaginatedDocsResponse struct {
	Documents    []DocStatusResponse `json:"documents"`
	Pagination   PaginationInfo      `json:"pagination"`
	StatusCounts map[string]int      `json:"status_counts"`
}

// ClearDocumentsResponse represents the response from the DELETE /documents endpoint.
type ClearDocumentsResponse struct {
	Status  string `json:"status"` // success, partial_success, busy, fail
	Message string `json:"message"`
}

// ScanResponse represents the response from the POST /documents/scan endpoint.
type ScanResponse struct {
	Status  string `json:"status"` // scanning_started
	Message string `json:"message"`
	TrackID string `json:"track_id"`
}

// UploadResponse represents the response from the POST /documents/upload endpoint.
type UploadResponse struct {
	Status  string `json:"status"` // success, duplicated, partial_success, failure
	Message string `json:"message"`
	TrackID string `json:"track_id"`
}

// StatusCountsResponse represents the response from the GET /documents/status_counts endpoint.
type StatusCountsResponse struct {
	StatusCounts map[string]int `json:"status_counts"`
}

// ReprocessResponse represents the response from the POST /documents/reprocess_failed endpoint.
type ReprocessResponse struct {
	Status  string `json:"status"` // reprocessing_started
	Message string `json:"message"`
}

// CancelPipelineResponse represents the response from the POST /documents/cancel_pipeline endpoint.
type CancelPipelineResponse struct {
	Status  string `json:"status"` // cancellation_requested, not_busy
	Message string `json:"message"`
}

// ClearCacheResponse represents the response from the POST /documents/clear_cache endpoint.
type ClearCacheResponse struct {
	Status  string `json:"status"` // success, fail
	Message string `json:"message"`
}

// PipelineStatusResponse represents the response from the GET /documents/pipeline_status endpoint.
type PipelineStatusResponse struct {
	Autoscanned     bool              `json:"autoscanned"`
	Busy            bool              `json:"busy"`
	JobName         string            `json:"job_name"`
	JobStart        string            `json:"job_start,omitempty"`
	Docs            int               `json:"docs"`
	Batchs          int               `json:"batchs"`
	CurBatch        int               `json:"cur_batch"`
	RequestPending  bool              `json:"request_pending"`
	LatestMessage   string            `json:"latest_message"`
	HistoryMessages []string          `json:"history_messages,omitempty"`
	UpdateStatus    map[string][]bool `json:"update_status"`
	Canceled        bool              `json:"cancellation_requested,omitempty"`
}

// TrackStatusResponse represents the response from the GET /documents/track_status/{track_id} endpoint.
type TrackStatusResponse struct {
	TrackID       string              `json:"track_id"`
	Documents     []DocStatusResponse `json:"documents"`
	TotalCount    int                 `json:"total_count"`
	StatusSummary map[string]int      `json:"status_summary"`
}

// QueryResponse represents the response from the query endpoint.
type QueryResponse struct {
	Response   string      `json:"response"`
	References []Reference `json:"references,omitempty"`
}

// StreamChunk represents a single chunk from the streaming query endpoint.
type StreamChunk struct {
	References []Reference `json:"references,omitempty"`
	Response   string      `json:"response,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Query sends a query request to the LightRAG API.
func (c *Client) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	ctx, span := c.tracer.Start(ctx, "Query", trace.WithAttributes(
		attribute.String("query", req.Query),
		attribute.String("mode", req.Mode),
	))
	defer span.End()

	url := fmt.Sprintf("%s/query", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &queryResp, nil
}

// QueryStream sends a streaming query request to the LightRAG API.
func (c *Client) QueryStream(ctx context.Context, req QueryRequest) (<-chan StreamChunk, error) {
	ctx, span := c.tracer.Start(ctx, "QueryStream", trace.WithAttributes(
		attribute.String("query", req.Query),
		attribute.String("mode", req.Mode),
	))

	url := fmt.Sprintf("%s/query/stream", c.baseURL)

	// Ensure stream is set to true for this endpoint
	req.Stream = true

	jsonData, err := json.Marshal(req)
	if err != nil {
		span.RecordError(err)
		span.End()
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		span.RecordError(err)
		span.End()
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		span.RecordError(err)
		span.End()
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		span.End()
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	ch := make(chan StreamChunk)

	go func() {
		defer resp.Body.Close()
		defer close(ch)
		defer span.End()

		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk StreamChunk
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					return
				}
				// If we have an error decoding a chunk, we send it in the error field
				// or just stop. Here we'll try to signal it.
				select {
				case ch <- StreamChunk{Error: fmt.Sprintf("decode error: %v", err)}:
				case <-ctx.Done():
				}
				return
			}

			select {
			case ch <- chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// GetDocuments retrieves the status of all documents (deprecated API).
func (c *Client) GetDocuments(ctx context.Context) (*DocsStatusesResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetDocuments")
	defer span.End()

	url := fmt.Sprintf("%s/documents", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var docsResp DocsStatusesResponse
	if err := json.NewDecoder(resp.Body).Decode(&docsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &docsResp, nil
}

// GetDocumentsPaginated retrieves documents with pagination support.
func (c *Client) GetDocumentsPaginated(ctx context.Context, req DocumentsRequest) (*PaginatedDocsResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetDocumentsPaginated", trace.WithAttributes(
		attribute.Int("page", req.Page),
		attribute.Int("page_size", req.PageSize),
	))
	defer span.End()

	url := fmt.Sprintf("%s/documents/paginated", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var docsResp PaginatedDocsResponse
	if err := json.NewDecoder(resp.Body).Decode(&docsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &docsResp, nil
}

// ClearDocuments deletes all documents from the system.
func (c *Client) ClearDocuments(ctx context.Context) (*ClearDocumentsResponse, error) {
	ctx, span := c.tracer.Start(ctx, "ClearDocuments")
	defer span.End()

	url := fmt.Sprintf("%s/documents", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var clearResp ClearDocumentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&clearResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &clearResp, nil
}

// ScanDocuments triggers a scan for new documents in the input directory.
func (c *Client) ScanDocuments(ctx context.Context) (*ScanResponse, error) {
	ctx, span := c.tracer.Start(ctx, "ScanDocuments")
	defer span.End()

	url := fmt.Sprintf("%s/documents/scan", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var scanResp ScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&scanResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &scanResp, nil
}

// GetPipelineStatus retrieves the current status of the document indexing pipeline.
func (c *Client) GetPipelineStatus(ctx context.Context) (*PipelineStatusResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetPipelineStatus")
	defer span.End()

	url := fmt.Sprintf("%s/documents/pipeline_status", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var pipeResp PipelineStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&pipeResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &pipeResp, nil
}

// GetTrackStatus retrieves the processing status of documents by tracking ID.
func (c *Client) GetTrackStatus(ctx context.Context, trackID string) (*TrackStatusResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetTrackStatus", trace.WithAttributes(
		attribute.String("track_id", trackID),
	))
	defer span.End()

	url := fmt.Sprintf("%s/documents/track_status/%s", c.baseURL, trackID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var trackResp TrackStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&trackResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &trackResp, nil
}

// UploadFile uploads a document to the LightRAG API.
func (c *Client) UploadFile(ctx context.Context, filename string, content io.Reader) (*UploadResponse, error) {
	ctx, span := c.tracer.Start(ctx, "UploadFile", trace.WithAttributes(
		attribute.String("filename", filename),
	))
	defer span.End()

	url := fmt.Sprintf("%s/documents/upload", c.baseURL)

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(fw, content); err != nil {
		return nil, fmt.Errorf("copy content: %w", err)
	}
	w.Close()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &b)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var uploadResp UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &uploadResp, nil
}

// GetStatusCounts retrieves the count of documents by status.
func (c *Client) GetStatusCounts(ctx context.Context) (*StatusCountsResponse, error) {
	ctx, span := c.tracer.Start(ctx, "GetStatusCounts")
	defer span.End()

	url := fmt.Sprintf("%s/documents/status_counts", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var countsResp StatusCountsResponse
	if err := json.NewDecoder(resp.Body).Decode(&countsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &countsResp, nil
}

// ReprocessFailedDocuments triggers reprocessing of failed and pending documents.
func (c *Client) ReprocessFailedDocuments(ctx context.Context) (*ReprocessResponse, error) {
	ctx, span := c.tracer.Start(ctx, "ReprocessFailedDocuments")
	defer span.End()

	url := fmt.Sprintf("%s/documents/reprocess_failed", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var reprocResp ReprocessResponse
	if err := json.NewDecoder(resp.Body).Decode(&reprocResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &reprocResp, nil
}

// CancelPipeline requests cancellation of the currently running pipeline.
func (c *Client) CancelPipeline(ctx context.Context) (*CancelPipelineResponse, error) {
	ctx, span := c.tracer.Start(ctx, "CancelPipeline")
	defer span.End()

	url := fmt.Sprintf("%s/documents/cancel_pipeline", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var cancelResp CancelPipelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&cancelResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &cancelResp, nil
}

// ClearCache clears all cached LLM responses.
func (c *Client) ClearCache(ctx context.Context) (*ClearCacheResponse, error) {
	ctx, span := c.tracer.Start(ctx, "ClearCache")
	defer span.End()

	url := fmt.Sprintf("%s/documents/clear_cache", c.baseURL)

	// The API expects a body for ClearCacheRequest even if ignored
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var clearResp ClearCacheResponse
	if err := json.NewDecoder(resp.Body).Decode(&clearResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &clearResp, nil
}
