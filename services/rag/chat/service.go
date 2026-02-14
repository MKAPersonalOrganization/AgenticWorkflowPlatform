package chat

import (
"bytes"
"context"
"encoding/json"
"fmt"
"io"
"net/http"
"time"

"school-with-ai/domain"
)

// Service handles chat operations against the Edgequake API.
type Service struct {
baseURL    string
httpClient *http.Client
}

// NewService creates a new Chat service.
func NewService(baseURL string, httpClient *http.Client) *Service {
if httpClient == nil {
httpClient = &http.Client{
Timeout: 30 * time.Second,
}
}
return &Service{
baseURL:    baseURL,
httpClient: httpClient,
}
}

// Complete sends a chat completion request and returns the response.
func (s *Service) Complete(ctx context.Context, tCtx *domain.TenantContext, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
urlStr := fmt.Sprintf("%s/api/v1/chat/completions", s.baseURL)

body, err := json.Marshal(req)
if err != nil {
return nil, fmt.Errorf("failed to marshal request: %w", err)
}

httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
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

var response ChatCompletionResponse
if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
return nil, fmt.Errorf("failed to decode response: %w", err)
}

return &response, nil
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
if resp.StatusCode >= 400 {
body, _ := io.ReadAll(resp.Body)
var errResp ErrorResponse
if json.Unmarshal(body, &errResp) == nil && errResp.Message != "" {
return fmt.Errorf("chat: %s (status %d)", errResp.Message, resp.StatusCode)
}
return fmt.Errorf("chat: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
}
return nil
}
