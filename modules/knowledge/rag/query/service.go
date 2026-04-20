package query

import (
	"aicompass/modules/knowledge/tenant"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Service provides access to the Edgequake Query API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Query service.
func NewService(baseURL string, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Service{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// Execute performs a standard RAG query.
func (s *Service) Execute(ctx context.Context, tCtx *tenant.TenantContext, req *QueryRequest) (*QueryResponse, error) {
	httpReq, err := s.buildQueryRequest(ctx, "/api/v1/query", req, tCtx, false)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var queryResp QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &queryResp, nil
}

// Stream performs a streaming RAG query with v2 structured JSON events.
func (s *Service) Stream(ctx context.Context, tCtx *tenant.TenantContext, req *StreamQueryRequest) (<-chan *QueryStreamEvent, <-chan error) {
	eventChan := make(chan *QueryStreamEvent)
	errChan := make(chan error, 1)
	go s.runStream(ctx, tCtx, req, eventChan, errChan)
	return eventChan, errChan
}

// runStream is the body of the Stream goroutine: build the SSE request,
// dispatch it, scan the server-sent events, and fan them out onto the
// caller's channels. Any fatal error lands on errChan; eventChan always
// closes on exit.
func (s *Service) runStream(
	ctx context.Context, tCtx *tenant.TenantContext, req *StreamQueryRequest,
	eventChan chan<- *QueryStreamEvent, errChan chan<- error,
) {
	defer close(eventChan)
	defer close(errChan)

	httpReq, err := s.buildQueryRequest(ctx, "/api/v1/query/stream", req, tCtx, true)
	if err != nil {
		errChan <- err
		return
	}
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		errChan <- fmt.Errorf("failed to execute request: %w", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errChan <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		return
	}
	if scanErr := scanSSEEvents(ctx, resp.Body, eventChan); scanErr != nil {
		errChan <- scanErr
	}
}

// buildQueryRequest marshals the request body and constructs the HTTP
// request with JSON content-type, the SSE Accept header when streaming,
// and the tenant-routing headers when the caller supplied a context.
// Extracted so Execute and Stream share one source of truth for the
// header set.
func (s *Service) buildQueryRequest(
	ctx context.Context, path string, reqBody any, tCtx *tenant.TenantContext, streaming bool,
) (*http.Request, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if streaming {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	applyTenantHeaders(httpReq, tCtx)
	return httpReq, nil
}

// applyTenantHeaders sets the X-Tenant-ID / X-Workspace-ID / X-User-ID /
// X-User-Role headers when the tenant context carries them. Nil context
// or nil/empty fields are silently skipped.
func applyTenantHeaders(req *http.Request, tCtx *tenant.TenantContext) {
	if tCtx == nil {
		return
	}
	if tCtx.TenantID != nil {
		req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
	}
	if tCtx.WorkspaceID != nil {
		req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
	}
	if tCtx.UserID != "" {
		req.Header.Set("X-User-ID", tCtx.UserID)
	}
	if tCtx.Role != "" {
		req.Header.Set("X-User-Role", tCtx.Role)
	}
}

// scanSSEEvents reads the Edgequake streaming-query response, parses each
// "data: <json>" frame, and forwards the decoded events. Terminal
// "[DONE]" frames and empty-data lines are skipped. Context cancellation
// and EOF close cleanly; other scanner errors surface to the caller.
func scanSSEEvents(ctx context.Context, body io.Reader, eventChan chan<- *QueryStreamEvent) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		content := strings.TrimPrefix(line, "data: ")
		if content == "" || content == "[DONE]" {
			continue
		}
		var event QueryStreamEvent
		if err := json.Unmarshal([]byte(content), &event); err != nil {
			return fmt.Errorf("failed to unmarshal event: %w", err)
		}
		eventChan <- &event
	}
	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	return nil
}
