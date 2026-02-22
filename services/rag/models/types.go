package models

import "encoding/json"

// ModelsListResponse represents the full models list.
type ModelsListResponse struct {
	Providers                []ProviderResponse `json:"providers"`
	DefaultLLMProvider       string             `json:"default_llm_provider"`
	DefaultLLMModel          string             `json:"default_llm_model"`
	DefaultEmbeddingProvider string             `json:"default_embedding_provider"`
	DefaultEmbeddingModel    string             `json:"default_embedding_model"`
}

// ProviderResponse represents a provider with its models.
type ProviderResponse struct {
	Name         string                  `json:"name"`
	DisplayName  string                  `json:"display_name"`
	ProviderType string                  `json:"provider_type"`
	Enabled      bool                    `json:"enabled"`
	Priority     uint32                  `json:"priority"`
	Description  string                  `json:"description"`
	Models       []ModelResponse         `json:"models"`
	Health       *ProviderHealthResponse `json:"health,omitempty"`
}

// ModelResponse represents a single model.
type ModelResponse struct {
	Name         string                    `json:"name"`
	DisplayName  string                    `json:"display_name"`
	ModelType    string                    `json:"model_type"`
	Description  string                    `json:"description"`
	Deprecated   bool                      `json:"deprecated"`
	Replacement  *string                   `json:"replacement,omitempty"`
	Capabilities ModelCapabilitiesResponse `json:"capabilities"`
	Cost         ModelCostResponse         `json:"cost"`
	Tags         []string                  `json:"tags"`
}

// ModelCapabilitiesResponse represents model capabilities.
type ModelCapabilitiesResponse struct {
	ContextLength           int  `json:"context_length"`
	MaxOutputTokens         int  `json:"max_output_tokens"`
	SupportsVision          bool `json:"supports_vision"`
	SupportsFunctionCalling bool `json:"supports_function_calling"`
	SupportsJSONMode        bool `json:"supports_json_mode"`
	SupportsStreaming       bool `json:"supports_streaming"`
	SupportsSystemMessage   bool `json:"supports_system_message"`
	EmbeddingDimension      int  `json:"embedding_dimension"`
}

// ModelCostResponse represents model cost per 1k tokens.
type ModelCostResponse struct {
	InputPer1K     float64 `json:"input_per_1k"`
	OutputPer1K    float64 `json:"output_per_1k"`
	EmbeddingPer1K float64 `json:"embedding_per_1k"`
	ImagePerUnit   float64 `json:"image_per_unit"`
}

// ProviderHealthResponse represents provider health status.
type ProviderHealthResponse struct {
	Available bool    `json:"available"`
	LatencyMS uint64  `json:"latency_ms"`
	Error     *string `json:"error,omitempty"`
	CheckedAt string  `json:"checked_at"`
}

// LlmModelsResponse represents the LLM models list.
type LlmModelsResponse struct {
	Models          []LlmModelItem `json:"models"`
	DefaultProvider string         `json:"default_provider"`
	DefaultModel    string         `json:"default_model"`
}

// LlmModelItem represents a single LLM model item.
type LlmModelItem struct {
	Provider            string `json:"provider"`
	ProviderDisplayName string `json:"provider_display_name"`
	// Flattened ModelResponse fields
	Name         string                    `json:"name"`
	DisplayName  string                    `json:"display_name"`
	ModelType    string                    `json:"model_type"`
	Description  string                    `json:"description"`
	Deprecated   bool                      `json:"deprecated"`
	Replacement  *string                   `json:"replacement,omitempty"`
	Capabilities ModelCapabilitiesResponse `json:"capabilities"`
	Cost         ModelCostResponse         `json:"cost"`
	Tags         []string                  `json:"tags"`
}

// EmbeddingModelsResponse represents the embedding models list.
type EmbeddingModelsResponse struct {
	Models          []EmbeddingModelItem `json:"models"`
	DefaultProvider string               `json:"default_provider"`
	DefaultModel    string               `json:"default_model"`
}

// EmbeddingModelItem represents a single embedding model item.
type EmbeddingModelItem struct {
	Provider            string `json:"provider"`
	ProviderDisplayName string `json:"provider_display_name"`
	Dimension           int    `json:"dimension"`
	// Flattened ModelResponse fields
	Name         string                    `json:"name"`
	DisplayName  string                    `json:"display_name"`
	ModelType    string                    `json:"model_type"`
	Description  string                    `json:"description"`
	Deprecated   bool                      `json:"deprecated"`
	Replacement  *string                   `json:"replacement,omitempty"`
	Capabilities ModelCapabilitiesResponse `json:"capabilities"`
	Cost         ModelCostResponse         `json:"cost"`
	Tags         []string                  `json:"tags"`
}

var _ = json.Marshal
