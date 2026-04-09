package messages

import (
	"aicompass/shared"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Service handles message operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Messages service.
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

// UpdateMessage updates a message by ID.
func (s *Service) UpdateMessage(ctx context.Context, tCtx *shared.TenantContext, messageID string, req UpdateMessageRequest) (*MessageResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/messages/%s", s.baseURL, messageID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// DeleteMessage deletes a message by ID.
func (s *Service) DeleteMessage(ctx context.Context, tCtx *shared.TenantContext, messageID string) error {
	urlStr := fmt.Sprintf("%s/api/v1/messages/%s", s.baseURL, messageID)

	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return err
	}

	return nil
}

func (s *Service) injectHeaders(req *http.Request, tCtx *shared.TenantContext) {
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
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("messages: not found")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("messages: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}
