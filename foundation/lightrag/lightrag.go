package lightrag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
