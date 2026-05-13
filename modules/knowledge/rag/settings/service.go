package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Service handles settings-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Settings service.
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

// GetStatus retrieves the provider status.
func (s *Service) GetStatus(ctx context.Context) (*ProviderStatusResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/settings/provider/status", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ProviderStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// GetProviders retrieves available providers.
func (s *Service) GetProviders(ctx context.Context) (*AvailableProvidersResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/settings/providers", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response AvailableProvidersResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

func (s *Service) handleError(resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("settings: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("settings: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
