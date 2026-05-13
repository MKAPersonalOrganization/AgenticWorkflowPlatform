package costs

import (
	"aicompass/modules/knowledge/tenant"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var (
	ErrUnauthorized = fmt.Errorf("costs: unauthorized")
	ErrForbidden    = fmt.Errorf("costs: access forbidden")
)

// Service handles cost-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Costs service.
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

// Summary retrieves the cost summary for the workspace.
func (s *Service) Summary(ctx context.Context, tCtx *tenant.TenantContext) (*WorkspaceCostSummaryResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/costs/summary", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response WorkspaceCostSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// History retrieves cost history.
func (s *Service) History(ctx context.Context, tCtx *tenant.TenantContext, params *CostHistoryParams) ([]CostHistoryPoint, error) {
	urlStr := fmt.Sprintf("%s/api/v1/costs/history", s.baseURL)

	query := url.Values{}
	if params != nil {
		if params.StartDate != nil {
			query.Set("start_date", *params.StartDate)
		}
		if params.EndDate != nil {
			query.Set("end_date", *params.EndDate)
		}
		if params.Granularity != nil {
			query.Set("granularity", *params.Granularity)
		}
	}

	if len(query) > 0 {
		urlStr += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response []CostHistoryPoint
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

// GetBudget retrieves the budget for the workspace.
func (s *Service) GetBudget(ctx context.Context, tCtx *tenant.TenantContext) (*BudgetInfo, error) {
	urlStr := fmt.Sprintf("%s/api/v1/costs/budget", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response BudgetInfo
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// UpdateBudget updates the budget for the workspace.
func (s *Service) UpdateBudget(ctx context.Context, tCtx *tenant.TenantContext, budget *BudgetInfo) (*BudgetInfo, error) {
	urlStr := fmt.Sprintf("%s/api/v1/costs/budget", s.baseURL)

	body, err := json.Marshal(budget)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response BudgetInfo
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
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}
}

func (s *Service) handleError(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("costs: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("costs: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
