package workspaces

// Extended service methods for additional workspace endpoints.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"aicompass/shared"

	"github.com/google/uuid"
)

// MetricsSnapshotTriggerResponse represents the result of triggering a snapshot.
type MetricsSnapshotTriggerResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func handleExtendedError(resp *http.Response) error {
	if resp.StatusCode >= 400 {
		return fmt.Errorf("workspaces: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// GetMetricsHistory retrieves metrics history for a workspace.
func (s *Service) GetMetricsHistory(ctx context.Context, tCtx *shared.TenantContext, workspaceID uuid.UUID) (*MetricsHistoryResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/workspaces/%s/metrics-history", s.baseURL, workspaceID)

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

	if err := handleExtendedError(resp); err != nil {
		return nil, err
	}

	var response MetricsHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// TriggerMetricsSnapshot triggers a manual metrics snapshot.
func (s *Service) TriggerMetricsSnapshot(ctx context.Context, tCtx *shared.TenantContext, workspaceID uuid.UUID) (*MetricsSnapshotTriggerResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/workspaces/%s/metrics-snapshot", s.baseURL, workspaceID)

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

	if err := handleExtendedError(resp); err != nil {
		return nil, err
	}

	var response MetricsSnapshotTriggerResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// GetBySlug retrieves a workspace by its slug.
func (s *Service) GetBySlug(ctx context.Context, tCtx *shared.TenantContext, slug string) (*WorkspaceResponse, error) {
	if tCtx.TenantID == nil {
		return nil, fmt.Errorf("tenant ID is required")
	}
	urlStr := fmt.Sprintf("%s/api/v1/tenants/%s/workspaces/by-slug/%s", s.baseURL, tCtx.TenantID, slug)

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

	if err := handleExtendedError(resp); err != nil {
		return nil, err
	}

	var response WorkspaceResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &response, nil
}

// ReprocessAllDocuments triggers reprocessing for all documents in the workspace.
func (s *Service) ReprocessAllDocuments(ctx context.Context, tCtx *shared.TenantContext, workspaceID uuid.UUID, req *ReprocessAllRequest) (*ReprocessAllResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/reprocess-documents", s.baseURL, workspaceID.String())

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response ReprocessAllResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}
