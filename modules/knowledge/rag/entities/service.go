package entities

import (
	"aicompass/modules/knowledge/tenant"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Service handles entity-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Entities service.
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

func injectHeaders(req *http.Request, tc tenant.TenantContext) {
	if tc.TenantID != nil {
		req.Header.Set("X-Tenant-ID", tc.TenantID.String())
	}
	if tc.WorkspaceID != nil {
		req.Header.Set("X-Workspace-ID", tc.WorkspaceID.String())
	}
	if tc.UserID != "" {
		req.Header.Set("X-User-ID", tc.UserID)
	}
	if tc.Role != "" {
		req.Header.Set("X-User-Role", tc.Role)
	}
}

func (s *Service) handleError(resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("entities: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("entities: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// ListEntities retrieves a paginated list of entities.
func (s *Service) ListEntities(ctx context.Context, tc tenant.TenantContext, params *ListEntitiesParams) (*ListEntitiesResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/entities", s.baseURL))
	if params != nil {
		q := u.Query()
		if params.Page > 0 {
			q.Set("page", strconv.Itoa(params.Page))
		}
		if params.PageSize > 0 {
			q.Set("page_size", strconv.Itoa(params.PageSize))
		}
		if params.EntityType != "" {
			q.Set("entity_type", params.EntityType)
		}
		if params.Search != "" {
			q.Set("search", params.Search)
		}
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ListEntitiesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// CreateEntity creates a new entity.
func (s *Service) CreateEntity(ctx context.Context, tc tenant.TenantContext, req CreateEntityRequest) (*CreateEntityResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/entities", s.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	injectHeaders(httpReq, tc)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response CreateEntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// EntityExists checks whether an entity with the given name exists.
func (s *Service) EntityExists(ctx context.Context, tc tenant.TenantContext, entityName string) (*EntityExistsResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/entities/exists", s.baseURL))
	q := u.Query()
	q.Set("entity_name", entityName)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response EntityExistsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// MergeEntities merges two entities together.
func (s *Service) MergeEntities(ctx context.Context, tc tenant.TenantContext, req MergeEntitiesRequest) (*MergeEntitiesResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/entities/merge", s.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	injectHeaders(httpReq, tc)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response MergeEntitiesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetEntity retrieves a specific entity by name.
func (s *Service) GetEntity(ctx context.Context, tc tenant.TenantContext, name string) (*GetEntityResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/entities/%s", s.baseURL, url.PathEscape(name))

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response GetEntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// UpdateEntity updates an existing entity by name.
func (s *Service) UpdateEntity(ctx context.Context, tc tenant.TenantContext, name string, req UpdateEntityRequest) (*UpdateEntityResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/entities/%s", s.baseURL, url.PathEscape(name))

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	injectHeaders(httpReq, tc)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response UpdateEntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// DeleteEntity deletes an entity by name.
func (s *Service) DeleteEntity(ctx context.Context, tc tenant.TenantContext, name string, confirm bool) (*DeleteEntityResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/entities/%s", s.baseURL, url.PathEscape(name)))
	q := u.Query()
	if confirm {
		q.Set("confirm", "true")
		q.Set("delete_relationships", "true")
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "DELETE", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response DeleteEntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetEntityNeighborhood retrieves the neighborhood graph for an entity.
func (s *Service) GetEntityNeighborhood(ctx context.Context, tc tenant.TenantContext, name string, depth int) (*EntityNeighborhoodResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/entities/%s/neighborhood", s.baseURL, url.PathEscape(name)))
	if depth > 0 {
		q := u.Query()
		q.Set("depth", strconv.Itoa(depth))
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response EntityNeighborhoodResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}
