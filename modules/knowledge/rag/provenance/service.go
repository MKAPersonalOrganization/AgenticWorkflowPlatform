package provenance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"school-with-ai/shared"
	"time"
)

// Service handles provenance-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Provenance service.
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

// GetEntityProvenance retrieves provenance information for an entity by ID.
func (s *Service) GetEntityProvenance(ctx context.Context, tCtx *shared.TenantContext, entityID string) (*EntityProvenanceResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/entities/%s/provenance", s.baseURL, url.PathEscape(entityID))

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

	var response EntityProvenanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
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
		return fmt.Errorf("provenance: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("provenance: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
