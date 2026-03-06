package domain

import (
	"context"
	"encoding/json"
	"time"
)

// AIProvider represents a supported AI model provider.
type AIProvider string

const (
	AIProviderOpenAI    AIProvider = "openai"
	AIProviderAnthropic AIProvider = "anthropic"
	AIProviderGemini    AIProvider = "gemini"
)

// SupportedProviders returns all supported AI providers.
func SupportedProviders() []AIProvider {
	return []AIProvider{
		AIProviderOpenAI,
		AIProviderAnthropic,
		AIProviderGemini,
	}
}

// ProviderModels returns the known models for each provider.
func ProviderModels(provider AIProvider) []string {
	switch provider {
	case AIProviderOpenAI:
		return []string{
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
			"gpt-4",
			"gpt-3.5-turbo",
			"o1",
			"o1-mini",
			"o3-mini",
		}
	case AIProviderAnthropic:
		return []string{
			"claude-sonnet-4-20250514",
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
		}
	case AIProviderGemini:
		return []string{
			"gemini-2.0-flash",
			"gemini-2.0-flash-lite",
			"gemini-1.5-pro",
			"gemini-1.5-flash",
		}
	default:
		return nil
	}
}

// ProviderEmbeddingModels returns the known embedding models for each provider.
func ProviderEmbeddingModels(provider AIProvider) []string {
	switch provider {
	case AIProviderOpenAI:
		return []string{
			"text-embedding-3-small",
			"text-embedding-3-large",
			"text-embedding-ada-002",
		}
	case AIProviderAnthropic:
		// Anthropic does not have embedding models
		return nil
	case AIProviderGemini:
		return []string{
			"text-embedding-004",
		}
	default:
		return nil
	}
}

// ProviderTTSModels returns the known text-to-speech models for each provider.
func ProviderTTSModels(provider AIProvider) []string {
	switch provider {
	case AIProviderOpenAI:
		return []string{
			"gpt-4o-mini-tts",
			"tts-1",
			"tts-1-hd",
		}
	default:
		// Only OpenAI supports TTS for now; more providers will be added later.
		return nil
	}
}

// TTSVoices returns the available voice options for a given TTS model.
func TTSVoices(model string) []string {
	switch model {
	case "gpt-4o-mini-tts":
		// Full voice set for the latest model.
		return []string{
			"alloy", "ash", "ballad", "coral", "echo",
			"fable", "nova", "onyx", "sage", "shimmer",
			"verse", "marin", "cedar",
		}
	case "tts-1", "tts-1-hd":
		// Smaller subset for legacy models.
		return []string{
			"alloy", "ash", "coral", "echo", "fable",
			"onyx", "nova", "sage", "shimmer",
		}
	default:
		// Sensible defaults.
		return []string{
			"alloy", "ash", "coral", "echo", "fable",
			"nova", "onyx", "sage", "shimmer",
		}
	}
}

// ProviderSupportsTTS reports whether the given provider has TTS capability.
func ProviderSupportsTTS(provider AIProvider) bool {
	return len(ProviderTTSModels(provider)) > 0
}

// AISettings represents per-user AI provider credentials.
// Each user has completely isolated settings.
// Capability-specific configuration (LLM model, TTS voice, embedding model, etc.)
// lives in AICapabilityConfig — providers only store credentials.
type AISettings struct {
	ID        string     `json:"id" db:"id"`
	UserID    string     `json:"user_id" db:"user_id"`
	Provider  AIProvider `json:"provider" db:"provider"`
	APIKey    string     `json:"api_key,omitempty" db:"api_key"`
	IsDefault bool       `json:"is_default" db:"is_default"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

// MaskedAPIKey returns the API key with all but the last 4 characters masked.
func (s *AISettings) MaskedAPIKey() string {
	if len(s.APIKey) <= 4 {
		return "****"
	}
	return "****" + s.APIKey[len(s.APIKey)-4:]
}

// AISettingsRepository defines persistence for user AI settings.
type AISettingsRepository interface {
	// GetByUserID returns all AI settings for a user.
	GetByUserID(ctx context.Context, userID string) ([]AISettings, error)

	// GetDefaultForUser returns the user's default AI settings.
	GetDefaultForUser(ctx context.Context, userID string) (*AISettings, error)

	// GetByID returns a specific AI settings entry.
	GetByID(ctx context.Context, id string) (*AISettings, error)

	// Upsert creates or updates an AI settings entry.
	Upsert(ctx context.Context, settings *AISettings) error

	// Delete removes an AI settings entry.
	Delete(ctx context.Context, id string, userID string) error

	// SetDefault marks one entry as default and unsets all others for the user.
	SetDefault(ctx context.Context, id string, userID string) error
}

// ═══════════════════════════════════════════════════════════════════════════════
// Capability-Based Configuration (Decoupled from Provider)
// ═══════════════════════════════════════════════════════════════════════════════

// AICapability represents a distinct AI function that can be independently configured.
type AICapability string

const (
	CapabilityLLM       AICapability = "llm"
	CapabilityTTS       AICapability = "tts"
	CapabilityEmbedding AICapability = "embedding"
	CapabilitySTT       AICapability = "stt"
	CapabilityImage     AICapability = "image"
)

// SupportedCapabilities returns all capabilities that can be configured.
func SupportedCapabilities() []AICapability {
	return []AICapability{
		CapabilityLLM,
		CapabilityTTS,
		CapabilityEmbedding,
	}
}

// AICapabilityConfig stores per-user, per-capability model assignment.
// A user can use OpenAI for LLM, Anthropic for embeddings, and OpenAI for TTS—
// each capability is independently wired to a provider + model + extra settings.
type AICapabilityConfig struct {
	ID         string          `json:"id" db:"id"`
	UserID     string          `json:"user_id" db:"user_id"`
	Capability AICapability    `json:"capability" db:"capability"`
	Provider   AIProvider      `json:"provider" db:"provider"`
	Model      string          `json:"model" db:"model"`
	Config     json.RawMessage `json:"config" db:"config"` // capability-specific (e.g. {voice, temperature})
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// TTSCapabilitySettings holds TTS-specific config stored in Config JSON.
type TTSCapabilitySettings struct {
	Voice string `json:"voice"`
}

// LLMCapabilitySettings holds LLM-specific config stored in Config JSON.
type LLMCapabilitySettings struct {
	MaxTokens     int     `json:"max_tokens,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	MaxIterations int     `json:"max_iterations,omitempty"`
}

// EmbeddingCapabilitySettings holds embedding-specific config stored in Config JSON.
type EmbeddingCapabilitySettings struct {
	Dimensions int `json:"dimensions,omitempty"`
}

// GetTTSSettings parses the Config JSON into TTSCapabilitySettings.
func (c *AICapabilityConfig) GetTTSSettings() TTSCapabilitySettings {
	var s TTSCapabilitySettings
	if len(c.Config) > 0 {
		_ = json.Unmarshal(c.Config, &s)
	}
	return s
}

// GetLLMSettings parses the Config JSON into LLMCapabilitySettings.
func (c *AICapabilityConfig) GetLLMSettings() LLMCapabilitySettings {
	var s LLMCapabilitySettings
	if len(c.Config) > 0 {
		_ = json.Unmarshal(c.Config, &s)
	}
	return s
}

// GetEmbeddingSettings parses the Config JSON into EmbeddingCapabilitySettings.
func (c *AICapabilityConfig) GetEmbeddingSettings() EmbeddingCapabilitySettings {
	var s EmbeddingCapabilitySettings
	if len(c.Config) > 0 {
		_ = json.Unmarshal(c.Config, &s)
	}
	return s
}

// AICapabilityConfigRepository provides persistence for capability configs.
type AICapabilityConfigRepository interface {
	// GetAllByUserID returns all capability configs for a user.
	GetAllByUserID(ctx context.Context, userID string) ([]AICapabilityConfig, error)

	// GetByCapability returns a specific capability config.
	GetByCapability(ctx context.Context, userID string, capability AICapability) (*AICapabilityConfig, error)

	// Upsert creates or updates a capability config.
	Upsert(ctx context.Context, config *AICapabilityConfig) error

	// Delete removes a capability config.
	Delete(ctx context.Context, userID string, capability AICapability) error
}
