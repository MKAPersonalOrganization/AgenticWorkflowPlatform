package aisettings

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
)

// Service provides business logic for per-user AI settings.
type Service struct {
	repo    domain.AISettingsRepository
	capRepo domain.AICapabilityConfigRepository
}

// New creates a new AI settings service.
func New(repo domain.AISettingsRepository, capRepo domain.AICapabilityConfigRepository) *Service {
	return &Service{repo: repo, capRepo: capRepo}
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

// ListTTSModels returns text-to-speech models for a given provider.
func (s *Service) ListTTSModels(provider domain.AIProvider) []string {
	return domain.ProviderTTSModels(provider)
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
// It reads the default provider for the API key, and the LLM capability config
// for the model/temperature/max_tokens. If no LLM capability is configured,
// it falls back to the default provider's first model.
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
		APIKey:   settings.APIKey,
	}

	// Check for an LLM capability config — it may override the provider and specify model/temperature/max_tokens
	capConfig, capErr := s.capRepo.GetByCapability(ctx, userID, domain.CapabilityLLM)
	if capErr == nil && capConfig != nil {
		cfg.Model = capConfig.Model

		// If the LLM capability points to a different provider, resolve its API key
		if capConfig.Provider != settings.Provider {
			allSettings, err := s.repo.GetByUserID(ctx, userID)
			if err == nil {
				for _, ps := range allSettings {
					if ps.Provider == capConfig.Provider && ps.APIKey != "" {
						cfg.Provider = string(capConfig.Provider)
						cfg.APIKey = ps.APIKey
						break
					}
				}
			}
		}

		llmSettings := capConfig.GetLLMSettings()
		if llmSettings.Temperature > 0 {
			t := llmSettings.Temperature
			cfg.Temperature = &t
		}
		if llmSettings.MaxIterations > 0 {
			cfg.MaxIterations = llmSettings.MaxIterations
		}
		if llmSettings.MaxTokens > 0 {
			maxTokens := llmSettings.MaxTokens
			cfg.MaxTokens = &maxTokens
		}
	} else {
		// No LLM capability configured — fall back to provider's first model
		models := domain.ProviderModels(settings.Provider)
		if len(models) > 0 {
			cfg.Model = models[0]
		}
	}

	return cfg, nil
}

// EmbeddingConfig holds the resolved embedding provider/model/dimension for a user.
type EmbeddingConfig struct {
	Provider  string
	Model     string
	Dimension int
	APIKey    string
}

// GetEmbeddingConfigForUser resolves the user's embedding config from the capability table.
// If no embedding capability is configured, falls back to the default provider's first embedding model.
func (s *Service) GetEmbeddingConfigForUser(ctx context.Context, userID string) (*EmbeddingConfig, error) {
	// Check capability config first
	capConfig, err := s.capRepo.GetByCapability(ctx, userID, domain.CapabilityEmbedding)
	if err != nil {
		return nil, fmt.Errorf("getting embedding capability config: %w", err)
	}

	if capConfig != nil {
		// Resolve API key from the matching provider
		settings, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("getting provider settings: %w", err)
		}

		var apiKey string
		for _, setting := range settings {
			if setting.Provider == capConfig.Provider && setting.APIKey != "" {
				apiKey = setting.APIKey
				break
			}
		}
		if apiKey == "" {
			return nil, fmt.Errorf("no API key for embedding provider %s", capConfig.Provider)
		}

		embSettings := capConfig.GetEmbeddingSettings()
		return &EmbeddingConfig{
			Provider:  string(capConfig.Provider),
			Model:     capConfig.Model,
			Dimension: embSettings.Dimensions,
			APIKey:    apiKey,
		}, nil
	}

	// Fallback: use the default provider's first available embedding model
	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user ai settings: %w", err)
	}

	// First: default provider with embedding models
	for _, st := range settings {
		if !st.IsDefault {
			continue
		}
		models := domain.ProviderEmbeddingModels(st.Provider)
		if len(models) > 0 {
			return &EmbeddingConfig{
				Provider: string(st.Provider),
				Model:    models[0],
				APIKey:   st.APIKey,
			}, nil
		}
	}

	// Second: any provider with embedding models
	for _, st := range settings {
		models := domain.ProviderEmbeddingModels(st.Provider)
		if len(models) > 0 {
			return &EmbeddingConfig{
				Provider: string(st.Provider),
				Model:    models[0],
				APIKey:   st.APIKey,
			}, nil
		}
	}

	return nil, nil // No embedding config available
}

// ═══════════════════════════════════════════════════════════════════════════════
// Capability-Based Configuration
// ═══════════════════════════════════════════════════════════════════════════════

// GetAllCapabilityConfigs returns all capability configs for a user.
func (s *Service) GetAllCapabilityConfigs(ctx context.Context, userID string) ([]domain.AICapabilityConfig, error) {
	configs, err := s.capRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting capability configs: %w", err)
	}
	return configs, nil
}

// GetCapabilityConfig returns a specific capability config for a user.
func (s *Service) GetCapabilityConfig(ctx context.Context, userID string, capability domain.AICapability) (*domain.AICapabilityConfig, error) {
	config, err := s.capRepo.GetByCapability(ctx, userID, capability)
	if err != nil {
		return nil, fmt.Errorf("getting capability config %s: %w", capability, err)
	}
	return config, nil
}

// SaveCapabilityConfig validates and upserts a capability config.
// It verifies that the referenced provider exists and has an API key.
func (s *Service) SaveCapabilityConfig(ctx context.Context, config *domain.AICapabilityConfig) error {
	if config.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if config.Capability == "" {
		return fmt.Errorf("capability is required")
	}
	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if config.Model == "" {
		return fmt.Errorf("model is required")
	}

	// Verify the provider is configured (has API key)
	settings, err := s.repo.GetByUserID(ctx, config.UserID)
	if err != nil {
		return fmt.Errorf("checking provider: %w", err)
	}

	providerFound := false
	for _, setting := range settings {
		if setting.Provider == config.Provider && setting.APIKey != "" {
			providerFound = true
			break
		}
	}
	if !providerFound {
		return fmt.Errorf("provider %s is not configured or missing API key — add it in the Providers section first", config.Provider)
	}

	if config.Config == nil {
		config.Config = json.RawMessage(`{}`)
	}

	return s.capRepo.Upsert(ctx, config)
}

// DeleteCapabilityConfig removes a capability config.
func (s *Service) DeleteCapabilityConfig(ctx context.Context, userID string, capability domain.AICapability) error {
	return s.capRepo.Delete(ctx, userID, capability)
}

// TTSConfig holds resolved TTS configuration for speech generation.
type TTSConfig struct {
	Provider string
	Model    string
	Voice    string
	APIKey   string
}

// GetTTSConfigForUser resolves the TTS configuration for a user
// from the capability config table.
func (s *Service) GetTTSConfigForUser(ctx context.Context, userID string) (*TTSConfig, error) {
	capConfig, err := s.capRepo.GetByCapability(ctx, userID, domain.CapabilityTTS)
	if err != nil {
		return nil, fmt.Errorf("getting TTS capability config: %w", err)
	}

	if capConfig == nil {
		return nil, nil // No TTS config
	}

	// Resolve API key from the provider
	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting provider settings: %w", err)
	}

	var apiKey string
	for _, setting := range settings {
		if setting.Provider == capConfig.Provider && setting.APIKey != "" {
			apiKey = setting.APIKey
			break
		}
	}
	if apiKey == "" {
		return nil, fmt.Errorf("no API key for TTS provider %s", capConfig.Provider)
	}

	ttsSettings := capConfig.GetTTSSettings()
	voice := ttsSettings.Voice
	if voice == "" {
		voice = "coral"
	}

	return &TTSConfig{
		Provider: string(capConfig.Provider),
		Model:    capConfig.Model,
		Voice:    voice,
		APIKey:   apiKey,
	}, nil
}

// GetTTSConfigForUserFlat returns the TTS config as flat values, satisfying the
// custom.TTSConfigResolver interface used by the runner adapter.
func (s *Service) GetTTSConfigForUserFlat(ctx context.Context, userID string) (apiKey, model, voice string, err error) {
	cfg, err := s.GetTTSConfigForUser(ctx, userID)
	if err != nil {
		return "", "", "", err
	}
	if cfg == nil {
		return "", "", "", nil
	}
	return cfg.APIKey, cfg.Model, cfg.Voice, nil
}
