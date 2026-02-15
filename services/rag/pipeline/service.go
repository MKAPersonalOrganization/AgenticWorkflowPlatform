package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"school-with-ai/domain"
)

var (
	ErrUnauthorized = fmt.Errorf("pipeline: unauthorized")
	ErrForbidden    = fmt.Errorf("pipeline: access forbidden")
)

// Service handles pipeline-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Pipeline service.
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

// Status retrieves the pipeline status.
func (s *Service) Status(ctx context.Context, tCtx *domain.TenantContext) (*EnhancedPipelineStatusResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/pipeline/status", s.baseURL)

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

	var response EnhancedPipelineStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Cancel cancels the currently running pipeline.
func (s *Service) Cancel(ctx context.Context, tCtx *domain.TenantContext) (*CancelPipelineResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/pipeline/cancel", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, nil)
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

	var response CancelPipelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Metrics retrieves queue metrics.
func (s *Service) Metrics(ctx context.Context, tCtx *domain.TenantContext) (*QueueMetricsResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/pipeline/queue-metrics", s.baseURL)

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

	var response QueueMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// History retrieves pipeline cost history.
func (s *Service) History(ctx context.Context, tCtx *domain.TenantContext, params *PipelineHistoryParams) ([]CostHistoryPoint, error) {
	urlStr := fmt.Sprintf("%s/api/v1/pipeline/history", s.baseURL)

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
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response []CostHistoryPoint
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

// EstimateCost estimates the cost for a given model and token count.
func (s *Service) EstimateCost(ctx context.Context, tCtx *domain.TenantContext, estReq *EstimateCostRequest) (*EstimateCostResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/pipeline/costs/estimate", s.baseURL)

	body, err := json.Marshal(estReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response EstimateCostResponse
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
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("pipeline: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("pipeline: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
