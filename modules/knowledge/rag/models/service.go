package models

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Service handles model-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Models service.
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

// List retrieves all available models grouped by provider.
func (s *Service) List(ctx context.Context) (*ModelsListResponse, error) {
	return doGet[ModelsListResponse](s, ctx, "/api/v1/models")
}

// ListLLM retrieves LLM models.
func (s *Service) ListLLM(ctx context.Context) (*LlmModelsResponse, error) {
	return doGet[LlmModelsResponse](s, ctx, "/api/v1/models/llm")
}

// ListEmbedding retrieves embedding models.
func (s *Service) ListEmbedding(ctx context.Context) (*EmbeddingModelsResponse, error) {
	return doGet[EmbeddingModelsResponse](s, ctx, "/api/v1/models/embedding")
}

// Health retrieves provider health status.
func (s *Service) Health(ctx context.Context) ([]ProviderHealthResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/models/health", s.baseURL)

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

	var response []ProviderHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}

// GetProvider retrieves a specific provider by name.
func (s *Service) GetProvider(ctx context.Context, providerName string) (*ProviderResponse, error) {
	return doGet[ProviderResponse](s, ctx, fmt.Sprintf("/api/v1/models/providers/%s", providerName))
}

// GetModel retrieves a specific model by provider and model name.
func (s *Service) GetModel(ctx context.Context, providerName, modelName string) (*ModelResponse, error) {
	return doGet[ModelResponse](s, ctx, fmt.Sprintf("/api/v1/models/providers/%s/models/%s", providerName, modelName))
}

func doGet[T any](s *Service, ctx context.Context, path string) (*T, error) {
	urlStr := fmt.Sprintf("%s%s", s.baseURL, path)

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

	var response T
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

func (s *Service) handleError(resp *http.Response) error {
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("models: not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("models: unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
