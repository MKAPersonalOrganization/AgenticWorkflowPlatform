package settings

import "encoding/json"

// ProviderStatusResponse represents the provider status.
type ProviderStatusResponse struct {
	Provider  LLMProviderStatus       `json:"provider"`
	Embedding EmbeddingProviderStatus `json:"embedding"`
	Storage   StorageStatus           `json:"storage"`
	Metadata  StatusMetadata          `json:"metadata"`
}

// LLMProviderStatus represents the LLM provider status.
type LLMProviderStatus struct {
	Name         string          `json:"name"`
	ProviderType string          `json:"type"`
	Status       string          `json:"status"`
	Model        string          `json:"model"`
	BaseURL      *string         `json:"base_url,omitempty"`
	Config       json.RawMessage `json:"config"`
}

// EmbeddingProviderStatus represents the embedding provider status.
type EmbeddingProviderStatus struct {
	Name         string `json:"name"`
	ProviderType string `json:"type"`
	Status       string `json:"status"`
	Model        string `json:"model"`
	Dimension    int    `json:"dimension"`
}

// StorageStatus represents storage status.
type StorageStatus struct {
	StorageType       string `json:"type"`
	Dimension         int    `json:"dimension"`
	DimensionMismatch bool   `json:"dimension_mismatch"`
	Namespace         string `json:"namespace"`
}

// StatusMetadata represents metadata about the status check.
type StatusMetadata struct {
	CheckedAt     string `json:"checked_at"`
	UptimeSeconds int    `json:"uptime_seconds"`
}

// AvailableProvidersResponse represents available providers.
type AvailableProvidersResponse struct {
	LLMProviders            []ProviderInfo `json:"llm_providers"`
	EmbeddingProviders      []ProviderInfo `json:"embedding_providers"`
	ActiveLLMProvider       string         `json:"active_llm_provider"`
	ActiveEmbeddingProvider string         `json:"active_embedding_provider"`
}

// ProviderInfo represents information about a provider.
type ProviderInfo struct {
	ID                 string              `json:"id"`
	Name               string              `json:"name"`
	Description        string              `json:"description"`
	Available          bool                `json:"available"`
	ConfigRequirements []ConfigRequirement `json:"config_requirements"`
	DefaultModels      DefaultModels       `json:"default_models"`
}

// ConfigRequirement represents a configuration requirement.
type ConfigRequirement struct {
	EnvVar      string `json:"env_var"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	Satisfied   bool   `json:"satisfied"`
}

// DefaultModels represents default model configuration.
type DefaultModels struct {
	ChatModel          string `json:"chat_model"`
	EmbeddingModel     string `json:"embedding_model"`
	EmbeddingDimension int    `json:"embedding_dimension"`
}
