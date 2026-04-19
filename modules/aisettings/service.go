package aisettings

import (
	"aicompass/foundation/agent"
	"context"
	"encoding/json"
	"fmt"
)

// service provides business logic for per-user AI settings.
type service struct {
	repo    AISettingsRepository
	capRepo AICapabilityConfigRepository
}

// newService creates a new AI settings service.
func newService(repo AISettingsRepository, capRepo AICapabilityConfigRepository) *service {
	return &service{repo: repo, capRepo: capRepo}
}

// listProviders returns all supported AI providers.
func (s *service) listProviders() []AIProvider {
	return SupportedProviders()
}

// listModels returns models for a given provider.
func (s *service) listModels(provider AIProvider) []string {
	return ProviderModels(provider)
}

// listEmbeddingModels returns embedding models for a given provider.
func (s *service) listEmbeddingModels(provider AIProvider) []string {
	return ProviderEmbeddingModels(provider)
}

// listTTSModels returns text-to-speech models for a given provider.
func (s *service) listTTSModels(provider AIProvider) []string {
	return ProviderTTSModels(provider)
}

// getUserSettings returns all AI settings for a user.
func (s *service) getUserSettings(ctx context.Context, userID string) ([]AISettings, error) {
	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user ai settings: %w", err)
	}
	return settings, nil
}

// saveSettings creates or updates an AI settings entry.
func (s *service) saveSettings(ctx context.Context, settings *AISettings) error {
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
	for _, p := range SupportedProviders() {
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

// deleteSettings removes an AI settings entry.
func (s *service) deleteSettings(ctx context.Context, id string, userID string) error {
	return s.repo.Delete(ctx, id, userID)
}

// setDefault marks an entry as the user's default provider.
func (s *service) setDefault(ctx context.Context, id string, userID string) error {
	return s.repo.SetDefault(ctx, id, userID)
}

// getModelConfigForUser resolves the active ModelConfig for a given user.
func (s *service) getModelConfigForUser(ctx context.Context, userID string) (*agent.ModelConfig, error) {
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

	capConfig, capErr := s.capRepo.GetByCapability(ctx, userID, CapabilityLLM)
	if capErr != nil || capConfig == nil {
		if models := ProviderModels(settings.Provider); len(models) > 0 {
			cfg.Model = models[0]
		}
		return cfg, nil
	}

	cfg.Model = capConfig.Model
	if capConfig.Provider != settings.Provider {
		s.remapToCapabilityProvider(ctx, userID, cfg, capConfig.Provider)
	}
	applyLLMSettings(cfg, capConfig.GetLLMSettings())

	return cfg, nil
}

// remapToCapabilityProvider rewrites cfg.Provider / cfg.APIKey when the LLM
// capability config targets a different provider than the user's default.
// Any lookup failure leaves cfg pointing at the default provider — this is
// a best-effort override, not a hard requirement.
func (s *service) remapToCapabilityProvider(ctx context.Context, userID string, cfg *agent.ModelConfig, provider AIProvider) {
	all, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return
	}
	if apiKey, ok := findAPIKeyForProvider(all, provider); ok {
		cfg.Provider = string(provider)
		cfg.APIKey = apiKey
	}
}

// applyLLMSettings copies non-zero sampling/iteration/limit fields from the
// capability config onto the ModelConfig. Zero-valued fields are left alone
// so the provider's own defaults win downstream.
func applyLLMSettings(cfg *agent.ModelConfig, llm LLMCapabilitySettings) {
	if llm.Temperature > 0 {
		t := llm.Temperature
		cfg.Temperature = &t
	}
	if llm.MaxIterations > 0 {
		cfg.MaxIterations = llm.MaxIterations
	}
	if llm.MaxTokens > 0 {
		mt := llm.MaxTokens
		cfg.MaxTokens = &mt
	}
}

// findAPIKeyForProvider scans the user's provider settings for a matching
// provider with a non-empty API key.
func findAPIKeyForProvider(settings []AISettings, provider AIProvider) (string, bool) {
	for _, st := range settings {
		if st.Provider == provider && st.APIKey != "" {
			return st.APIKey, true
		}
	}
	return "", false
}

// getAPIKeyForProvider returns the stored API key for a specific provider.
// Used when a per-agent model override selects a different provider than the user's default.
func (s *service) getAPIKeyForProvider(ctx context.Context, userID, provider string) (string, error) {
	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("getting user settings: %w", err)
	}
	for _, st := range settings {
		if string(st.Provider) == provider && st.APIKey != "" {
			return st.APIKey, nil
		}
	}
	return "", fmt.Errorf("provider %q is not configured or missing API key — add it in Settings › AI", provider)
}

// listModelsFromProvider fetches the available chat models for the given provider
// by calling the provider's model-listing API with the user's stored API key.
func (s *service) listModelsFromProvider(ctx context.Context, userID, provider string) ([]ProviderModel, error) {
	apiKey, err := s.getAPIKeyForProvider(ctx, userID, provider)
	if err != nil {
		return nil, err
	}
	switch AIProvider(provider) {
	case AIProviderGemini:
		return fetchGeminiModels(ctx, apiKey)
	case AIProviderOpenAI:
		return fetchOpenAIModels(ctx, apiKey)
	case AIProviderAnthropic:
		return fetchAnthropicModels(ctx, apiKey)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// EmbeddingConfig holds the resolved embedding provider/model/dimension for a user.
type EmbeddingConfig struct {
	Provider  string
	Model     string
	Dimension int
	APIKey    string
}

// getEmbeddingConfigForUser resolves the user's embedding config from the capability table.
func (s *service) getEmbeddingConfigForUser(ctx context.Context, userID string) (*EmbeddingConfig, error) {
	capConfig, err := s.capRepo.GetByCapability(ctx, userID, CapabilityEmbedding)
	if err != nil {
		return nil, fmt.Errorf("getting embedding capability config: %w", err)
	}

	settings, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user ai settings: %w", err)
	}

	if capConfig != nil {
		return embeddingConfigFromCapability(capConfig, settings)
	}
	return embeddingConfigFromProviderDefaults(settings), nil
}

// embeddingConfigFromCapability builds the EmbeddingConfig when the user has
// a CapabilityEmbedding row set — the provider/model come from the capability,
// and the API key is resolved by scanning the provider settings list.
func embeddingConfigFromCapability(capConfig *AICapabilityConfig, settings []AISettings) (*EmbeddingConfig, error) {
	apiKey, ok := findAPIKeyForProvider(settings, capConfig.Provider)
	if !ok {
		return nil, fmt.Errorf("no API key for embedding provider %s", capConfig.Provider)
	}
	emb := capConfig.GetEmbeddingSettings()
	return &EmbeddingConfig{
		Provider:  string(capConfig.Provider),
		Model:     capConfig.Model,
		Dimension: emb.Dimensions,
		APIKey:    apiKey,
	}, nil
}

// embeddingConfigFromProviderDefaults falls back to the first provider with
// embedding models: default provider first, then any provider. Returns nil
// when no provider exposes embedding models.
func embeddingConfigFromProviderDefaults(settings []AISettings) *EmbeddingConfig {
	if cfg := firstEmbeddingProviderMatching(settings, func(st AISettings) bool { return st.IsDefault }); cfg != nil {
		return cfg
	}
	return firstEmbeddingProviderMatching(settings, func(AISettings) bool { return true })
}

func firstEmbeddingProviderMatching(settings []AISettings, match func(AISettings) bool) *EmbeddingConfig {
	for _, st := range settings {
		if !match(st) {
			continue
		}
		models := ProviderEmbeddingModels(st.Provider)
		if len(models) == 0 {
			continue
		}
		return &EmbeddingConfig{
			Provider: string(st.Provider),
			Model:    models[0],
			APIKey:   st.APIKey,
		}
	}
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Capability-Based Configuration
// ═══════════════════════════════════════════════════════════════════════════════

// getAllCapabilityConfigs returns all capability configs for a user.
func (s *service) getAllCapabilityConfigs(ctx context.Context, userID string) ([]AICapabilityConfig, error) {
	configs, err := s.capRepo.GetAllByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting capability configs: %w", err)
	}
	return configs, nil
}

// saveCapabilityConfig validates and upserts a capability config.
func (s *service) saveCapabilityConfig(ctx context.Context, config *AICapabilityConfig) error {
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

// deleteCapabilityConfig removes a capability config.
func (s *service) deleteCapabilityConfig(ctx context.Context, userID string, capability AICapability) error {
	return s.capRepo.Delete(ctx, userID, capability)
}

// ttsConfig holds resolved TTS configuration for speech generation.
type ttsConfig struct {
	Provider string
	Model    string
	Voice    string
	APIKey   string
}

// getTTSConfigForUser resolves the TTS configuration for a user.
func (s *service) getTTSConfigForUser(ctx context.Context, userID string) (*ttsConfig, error) {
	capConfig, err := s.capRepo.GetByCapability(ctx, userID, CapabilityTTS)
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

	return &ttsConfig{
		Provider: string(capConfig.Provider),
		Model:    capConfig.Model,
		Voice:    voice,
		APIKey:   apiKey,
	}, nil
}

// getTTSConfigForUserFlat returns the TTS config as flat values, satisfying the
// custom.TTSConfigResolver interface used by the runner adapter.
func (s *service) getTTSConfigForUserFlat(ctx context.Context, userID string) (apiKey, model, voice string, err error) {
	cfg, err := s.getTTSConfigForUser(ctx, userID)
	if err != nil {
		return "", "", "", err
	}
	if cfg == nil {
		return "", "", "", nil
	}
	return cfg.APIKey, cfg.Model, cfg.Voice, nil
}
