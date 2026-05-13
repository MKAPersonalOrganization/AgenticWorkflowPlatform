package workspaces

import (
	"aicompass/modules/knowledge/tenant"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// Service provides access to the Workspace-related endpoints of the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Workspace service.
func NewService(baseURL string, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Service{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// List retrieves a list of workspaces for a specific tenant.
func (s *Service) List(ctx context.Context, tCtx *tenant.TenantContext, page, pageSize int) (*WorkspaceListResponse, error) {
	if tCtx == nil || tCtx.TenantID == nil {
		return nil, fmt.Errorf("tenant ID is required")
	}

	offset := (page - 1) * pageSize
	url := fmt.Sprintf("%s/api/v1/tenants/%s/workspaces?limit=%d&offset=%d", s.baseURL, tCtx.TenantID.String(), pageSize, offset)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Inject tenant context headers
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
		req.Header.Set("X-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response WorkspaceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Create creates a new workspace for a specific tenant.
func (s *Service) Create(ctx context.Context, tCtx *tenant.TenantContext, req *CreateWorkspaceRequest) (*WorkspaceResponse, error) {
	if tCtx == nil || tCtx.TenantID == nil {
		return nil, fmt.Errorf("tenant ID is required")
	}

	url := fmt.Sprintf("%s/api/v1/tenants/%s/workspaces", s.baseURL, tCtx.TenantID.String())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Inject tenant context headers
	if tCtx.TenantID != nil {
		httpReq.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
	}
	if tCtx.UserID != "" {
		httpReq.Header.Set("X-User-ID", tCtx.UserID)
	}
	if tCtx.Role != "" {
		httpReq.Header.Set("X-Role", tCtx.Role)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response WorkspaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Get retrieves a specific workspace by its ID.
func (s *Service) Get(ctx context.Context, tCtx *tenant.TenantContext, workspaceID uuid.UUID) (*WorkspaceResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s", s.baseURL, workspaceID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.UserID != "" {
			req.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			req.Header.Set("X-Role", tCtx.Role)
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("workspace not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response WorkspaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Delete removes a specific workspace by its ID.
func (s *Service) Delete(ctx context.Context, tCtx *tenant.TenantContext, workspaceID uuid.UUID) error {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s", s.baseURL, workspaceID.String())

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.UserID != "" {
			req.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			req.Header.Set("X-Role", tCtx.Role)
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil // Idempotent
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// GetStats retrieves statistics for a specific workspace.
func (s *Service) GetStats(ctx context.Context, tCtx *tenant.TenantContext, workspaceID uuid.UUID) (*WorkspaceStatsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/stats", s.baseURL, workspaceID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.UserID != "" {
			req.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			req.Header.Set("X-Role", tCtx.Role)
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("workspace not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response WorkspaceStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Update modifies an existing workspace's fields.
func (s *Service) Update(ctx context.Context, tCtx *tenant.TenantContext, workspaceID uuid.UUID, req *UpdateWorkspaceRequest) (*WorkspaceResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s", s.baseURL, workspaceID.String())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Inject tenant context headers
	if tCtx != nil {
		if tCtx.TenantID != nil {
			httpReq.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.UserID != "" {
			httpReq.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			httpReq.Header.Set("X-Role", tCtx.Role)
		}
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("workspace not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response WorkspaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// RebuildGraph clears the knowledge graph and triggers a rebuild.
func (s *Service) RebuildGraph(ctx context.Context, tCtx *tenant.TenantContext, workspaceID uuid.UUID, req *RebuildKnowledgeGraphRequest) (*RebuildKnowledgeGraphResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/rebuild-knowledge-graph", s.baseURL, workspaceID.String())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response RebuildKnowledgeGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// RebuildEmbeddings clears all embeddings and triggers a re-indexing.
func (s *Service) RebuildEmbeddings(ctx context.Context, tCtx *tenant.TenantContext, workspaceID uuid.UUID, req *RebuildEmbeddingsRequest) (*RebuildEmbeddingsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/rebuild-embeddings", s.baseURL, workspaceID.String())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response RebuildEmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

func (s *Service) injectHeaders(req *http.Request, tCtx *tenant.TenantContext) {
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.UserID != "" {
			req.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			req.Header.Set("X-Role", tCtx.Role)
		}
	}
}
