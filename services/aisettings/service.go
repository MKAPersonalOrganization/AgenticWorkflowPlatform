package aisettings

import (
	"context"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
)

// Service provides business logic for per-user AI settings.
type Service struct {
	repo domain.AISettingsRepository
}

// New creates a new AI settings service.
func New(repo domain.AISettingsRepository) *Service {
	return &Service{repo: repo}
}

// ListProviders returns all supported AI providers.
func (s *Service) ListProviders() []domain.AIProvider {
	return domain.SupportedProviders()
}

// ListModels returns models for a given provider.
func (s *Service) ListModels(provider domain.AIProvider) []string {
	return domain.ProviderModels(provider)
}

// ListEmbeddingModels returns embedding models for a given provider.
func (s *Service) ListEmbeddingModels(provider domain.AIProvider) []string {
	return domain.ProviderEmbeddingModels(provider)
}

// GetUserSettings returns all AI settings for a user.
func (s *Service) GetUserSettings(ctx context.Context, userID string) ([]domain.AISettings, error) {
	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user ai settings: %w", err)
	}
	return settings, nil
}

// GetDefaultSettings returns the user's default AI settings.
func (s *Service) GetDefaultSettings(ctx context.Context, userID string) (*domain.AISettings, error) {
	settings, err := s.repo.GetDefaultForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting default ai settings: %w", err)
	}
	return settings, nil
}

// SaveSettings creates or updates an AI settings entry.
func (s *Service) SaveSettings(ctx context.Context, settings *domain.AISettings) error {
	if settings.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if settings.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if settings.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}

	// Validate provider
	valid := false
	for _, p := range domain.SupportedProviders() {
		if p == settings.Provider {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unsupported provider: %s", settings.Provider)
	}

	// Check if this is the first setting for the user — make it default
	existing, err := s.repo.GetByUserID(ctx, settings.UserID)
	if err != nil {
		return fmt.Errorf("checking existing settings: %w", err)
	}
	if len(existing) == 0 {
		settings.IsDefault = true
	}

	if err := s.repo.Upsert(ctx, settings); err != nil {
		return fmt.Errorf("saving ai settings: %w", err)
	}

	return nil
}

// DeleteSettings removes an AI settings entry.
func (s *Service) DeleteSettings(ctx context.Context, id string, userID string) error {
	return s.repo.Delete(ctx, id, userID)
}

// SetDefault marks an entry as the user's default provider.
func (s *Service) SetDefault(ctx context.Context, id string, userID string) error {
	return s.repo.SetDefault(ctx, id, userID)
}

// GetModelConfigForUser resolves the active ModelConfig for a given user.
// This is used by the runner/orchestrator to get per-user credentials.
func (s *Service) GetModelConfigForUser(ctx context.Context, userID string) (*agent.ModelConfig, error) {
	settings, err := s.repo.GetDefaultForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting default ai settings for user %s: %w", userID, err)
	}
	if settings == nil {
		return nil, fmt.Errorf("no AI settings configured for user %s: please configure your AI provider in Settings > AI", userID)
	}
	if settings.APIKey == "" {
		return nil, fmt.Errorf("no API key configured for provider %s: please set your API key in Settings > AI", settings.Provider)
	}

	cfg := &agent.ModelConfig{
		Provider: string(settings.Provider),
		Model:    settings.DefaultLLMModel,
		APIKey:   settings.APIKey,
	}

	if settings.MaxTokens > 0 || settings.Temperature > 0 {
		cfg.AdditionalConfig = map[string]any{}
		if settings.MaxTokens > 0 {
			cfg.AdditionalConfig["max_tokens"] = settings.MaxTokens
		}
		if settings.Temperature > 0 {
			cfg.AdditionalConfig["temperature"] = settings.Temperature
		}
	}

	return cfg, nil
}
