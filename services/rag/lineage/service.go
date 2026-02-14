package lineage

import (
"context"
"encoding/json"
"fmt"
"net/http"
"net/url"
"time"

"school-with-ai/domain"
)

// Service handles lineage-related operations against the Edgequake API.
type Service struct {
baseURL    string
httpClient *http.Client
}

// NewService creates a new Lineage service.
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

// GetEntityLineage retrieves the lineage information for a specific entity.
func (s *Service) GetEntityLineage(ctx context.Context, tCtx *domain.TenantContext, entityName string) (*EntityLineageResponse, error) {
urlStr := fmt.Sprintf("%s/api/v1/lineage/entities/%s", s.baseURL, url.PathEscape(entityName))

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

var response EntityLineageResponse
if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
return nil, fmt.Errorf("failed to decode response: %w", err)
}

return &response, nil
}

// GetDocumentLineage retrieves the graph lineage information for a specific document.
func (s *Service) GetDocumentLineage(ctx context.Context, tCtx *domain.TenantContext, documentID string) (*DocumentGraphLineageResponse, error) {
urlStr := fmt.Sprintf("%s/api/v1/lineage/documents/%s", s.baseURL, url.PathEscape(documentID))

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

var response DocumentGraphLineageResponse
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
if resp.StatusCode == http.StatusNotFound {
return fmt.Errorf("lineage: not found")
}
if resp.StatusCode >= 400 {
return fmt.Errorf("lineage: unexpected status code: %d", resp.StatusCode)
}
return nil
}
