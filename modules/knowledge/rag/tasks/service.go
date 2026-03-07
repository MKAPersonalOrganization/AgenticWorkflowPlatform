package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"school-with-ai/shared"
	"time"
)

var (
	ErrUnauthorized = fmt.Errorf("tasks: unauthorized")
	ErrForbidden    = fmt.Errorf("tasks: access forbidden")
)

// Service handles task-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Task service.
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

// Get retrieves a task by its track ID.
func (s *Service) Get(ctx context.Context, tCtx *shared.TenantContext, trackID string) (*TaskResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/tasks/%s", s.baseURL, trackID)

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

	var response TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// List retrieves a list of tasks with filters and pagination.
func (s *Service) List(ctx context.Context, tCtx *shared.TenantContext, params *ListTasksParams) (*TaskListResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/tasks", s.baseURL)

	// Build query parameters
	query := url.Values{}
	if params != nil {
		if params.TenantID != nil {
			query.Add("tenant_id", params.TenantID.String())
		}
		if params.WorkspaceID != nil {
			query.Add("workspace_id", params.WorkspaceID.String())
		}
		if params.Status != nil {
			query.Add("status", *params.Status)
		}
		if params.TaskType != nil {
			query.Add("task_type", *params.TaskType)
		}
		if params.Page != nil {
			query.Add("page", fmt.Sprintf("%d", *params.Page))
		}
		if params.PageSize != nil {
			query.Add("page_size", fmt.Sprintf("%d", *params.PageSize))
		}
		if params.Sort != nil {
			query.Add("sort", *params.Sort)
		}
		if params.Order != nil {
			query.Add("order", *params.Order)
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

	var response TaskListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Cancel cancels a task by its track ID.
func (s *Service) Cancel(ctx context.Context, tCtx *shared.TenantContext, trackID string) (*TaskResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/tasks/%s/cancel", s.baseURL, trackID)

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader([]byte{}))
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

	var response TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Retry retries a failed task by its track ID.
func (s *Service) Retry(ctx context.Context, tCtx *shared.TenantContext, trackID string) (*TaskResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/tasks/%s/retry", s.baseURL, trackID)

	req, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader([]byte{}))
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

	var response TaskResponse
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
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("task not found")
	}
	if resp.StatusCode == http.StatusConflict {
		return fmt.Errorf("task operation conflict")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
