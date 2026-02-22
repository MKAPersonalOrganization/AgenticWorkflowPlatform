package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Service provides access to the EdgeQuake health endpoints.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Health service.
func NewService(baseURL string, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Service{baseURL: baseURL, httpClient: httpClient}
}

// Check calls GET /health and returns the health response.
func (s *Service) Check(ctx context.Context) (*HealthResponse, error) {
	urlStr := fmt.Sprintf("%s/health", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("health: failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("health: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health: unexpected status %d", resp.StatusCode)
	}

	var result HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("health: failed to decode response: %w", err)
	}
	return &result, nil
}

// Ready calls GET /ready and returns nil if the service is ready.
func (s *Service) Ready(ctx context.Context) error {
	urlStr := fmt.Sprintf("%s/ready", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("ready: failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ready: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ready: not ready (status %d)", resp.StatusCode)
	}
	return nil
}

// Live calls GET /live and returns nil if the service is alive.
func (s *Service) Live(ctx context.Context) error {
	urlStr := fmt.Sprintf("%s/live", s.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("live: failed to create request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("live: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("live: not alive (status %d)", resp.StatusCode)
	}
	return nil
}
