package query

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"school-with-ai/domain"
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
func (s *Service) Execute(ctx context.Context, tCtx *domain.TenantContext, req *QueryRequest) (*QueryResponse, error) {
	url := fmt.Sprintf("%s/api/v1/query", s.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if tCtx != nil {
		if tCtx.TenantID != nil {
			httpReq.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			httpReq.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		if tCtx.UserID != "" {
			httpReq.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			httpReq.Header.Set("X-User-Role", tCtx.Role)
		}
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

// Stream performs a streaming RAG query.
func (s *Service) Stream(ctx context.Context, tCtx *domain.TenantContext, req *StreamQueryRequest) (<-chan string, <-chan error) {
	textChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		defer close(textChan)
		defer close(errChan)

		url := fmt.Sprintf("%s/api/v1/query/stream", s.baseURL)

		body, err := json.Marshal(req)
		if err != nil {
			errChan <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			errChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		if tCtx != nil {
			if tCtx.TenantID != nil {
				httpReq.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
			}
			if tCtx.WorkspaceID != nil {
				httpReq.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
			}
			if tCtx.UserID != "" {
				httpReq.Header.Set("X-User-ID", tCtx.UserID)
			}
			if tCtx.Role != "" {
				httpReq.Header.Set("X-User-Role", tCtx.Role)
			}
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

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				content := strings.TrimPrefix(line, "data: ")
				if content != "" {
					textChan <- content
				}
			}
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF && ctx.Err() == nil {
				errChan <- fmt.Errorf("scanner error: %w", err)
			}
		}
	}()

	return textChan, errChan
}
