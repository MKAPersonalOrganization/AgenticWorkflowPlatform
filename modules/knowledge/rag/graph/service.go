package graph

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

// Service handles graph-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Graph service.
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
		return fmt.Errorf("graph: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("graph: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// GetGraph retrieves the knowledge graph.
func (s *Service) GetGraph(ctx context.Context, tc tenant.TenantContext, params *GraphQueryParams) (*KnowledgeGraphResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph", s.baseURL))
	if params != nil {
		q := u.Query()
		if params.StartNode != nil {
			q.Set("start_node", *params.StartNode)
		}
		if params.Depth > 0 {
			q.Set("depth", strconv.Itoa(params.Depth))
		}
		if params.MaxNodes > 0 {
			q.Set("max_nodes", strconv.Itoa(params.MaxNodes))
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

	var response KnowledgeGraphResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetNode retrieves a specific graph node.
func (s *Service) GetNode(ctx context.Context, tc tenant.TenantContext, nodeID string) (*GraphNodeResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/graph/nodes/%s", s.baseURL, url.PathEscape(nodeID))

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

	var response GraphNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// SearchLabels searches for labels matching a query.
func (s *Service) SearchLabels(ctx context.Context, tc tenant.TenantContext, q string, limit int) (*SearchLabelsResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/labels/search", s.baseURL))
	qs := u.Query()
	qs.Set("q", q)
	if limit > 0 {
		qs.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = qs.Encode()

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

	var response SearchLabelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// SearchNodes searches for nodes matching a query.
func (s *Service) SearchNodes(ctx context.Context, tc tenant.TenantContext, q string, limit int) (*SearchNodesResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/nodes/search", s.baseURL))
	qs := u.Query()
	qs.Set("q", q)
	if limit > 0 {
		qs.Set("limit", strconv.Itoa(limit))
	}
	u.RawQuery = qs.Encode()

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

	var response SearchNodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetPopularLabels retrieves popular labels.
func (s *Service) GetPopularLabels(ctx context.Context, tc tenant.TenantContext, limit int) (*PopularLabelsResponse, error) {
	u, _ := url.Parse(fmt.Sprintf("%s/api/v1/graph/labels/popular", s.baseURL))
	if limit > 0 {
		qs := u.Query()
		qs.Set("limit", strconv.Itoa(limit))
		u.RawQuery = qs.Encode()
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

	var response PopularLabelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetDegreesBatch retrieves degrees for multiple nodes.
func (s *Service) GetDegreesBatch(ctx context.Context, tc tenant.TenantContext, nodeIDs []string) (*BatchDegreeResponse, error) {
	body, err := json.Marshal(BatchDegreeRequest{NodeIDs: nodeIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	urlStr := fmt.Sprintf("%s/api/v1/graph/degrees/batch", s.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	injectHeaders(req, tc)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response BatchDegreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}
