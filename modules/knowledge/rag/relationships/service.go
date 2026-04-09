package relationships

import (
	"aicompass/shared"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Service handles relationship-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Relationships service.
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

func injectHeaders(req *http.Request, tc shared.TenantContext) {
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
		return fmt.Errorf("relationships: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("relationships: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// ListRelationships retrieves a paginated list of relationships.
func (s *Service) ListRelationships(ctx context.Context, tc shared.TenantContext, params *ListRelationshipsParams) (*ListRelationshipsResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/relationships", s.baseURL))
	if params != nil {
		q := u.Query()
		if params.Page > 0 {
			q.Set("page", strconv.Itoa(params.Page))
		}
		if params.PageSize > 0 {
			q.Set("page_size", strconv.Itoa(params.PageSize))
		}
		if params.RelationshipType != "" {
			q.Set("relationship_type", params.RelationshipType)
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
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ListRelationshipsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// CreateRelationship creates a new relationship.
func (s *Service) CreateRelationship(ctx context.Context, tc shared.TenantContext, req CreateRelationshipRequest) (*CreateRelationshipResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/relationships", s.baseURL)

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
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response CreateRelationshipResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetRelationship retrieves a specific relationship by ID.
func (s *Service) GetRelationship(ctx context.Context, tc shared.TenantContext, id string) (*GetRelationshipResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/relationships/%s", s.baseURL, url.PathEscape(id))

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response GetRelationshipResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// UpdateRelationship updates an existing relationship by ID.
func (s *Service) UpdateRelationship(ctx context.Context, tc shared.TenantContext, id string, req UpdateRelationshipRequest) (*UpdateRelationshipResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/relationships/%s", s.baseURL, url.PathEscape(id))

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
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response UpdateRelationshipResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// DeleteRelationship deletes a relationship by ID.
func (s *Service) DeleteRelationship(ctx context.Context, tc shared.TenantContext, id string) (*DeleteRelationshipResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/relationships/%s", s.baseURL, url.PathEscape(id))

	req, err := http.NewRequestWithContext(ctx, "DELETE", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response DeleteRelationshipResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}
