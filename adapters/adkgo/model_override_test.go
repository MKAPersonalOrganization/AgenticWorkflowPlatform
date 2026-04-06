package adkgo

import (
	"context"
	"errors"
	"testing"

	fagent "aicompass/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// stubAPIKeyResolver is a test double for APIKeyResolver.
// It returns the pre-seeded key for a provider, or an error when the provider
// has not been configured by the user.
// ──────────────────────────────────────────────────────────────────────────────

type stubAPIKeyResolver struct {
	configuredKeys map[string]string // provider → apiKey
}

func (s *stubAPIKeyResolver) GetAPIKeyForProvider(_ context.Context, _, provider string) (string, error) {
	if apiKey, ok := s.configuredKeys[provider]; ok {
		return apiKey, nil
	}
	return "", errors.New("provider not configured: " + provider)
}

// resolverWithAllProviders returns a stub where gemini, openai, and anthropic
// all have API keys — the typical "user has configured all three providers" scenario.
func resolverWithAllProviders() *stubAPIKeyResolver {
	return &stubAPIKeyResolver{configuredKeys: map[string]string{
		"gemini":    "gemini-api-key",
		"openai":    "openai-api-key",
		"anthropic": "anthropic-api-key",
	}}
}

// userGlobalGeminiConfig returns the model config a user has set as their global
// default: Gemini Flash with temperature 0.7, 4096 max tokens, 5 max iterations.
func userGlobalGeminiConfig() *fagent.ModelConfig {
	return &fagent.ModelConfig{
		Provider:      "gemini",
		Model:         "gemini-2.0-flash",
		APIKey:        "gemini-api-key",
		Temperature:   f64(0.7),
		MaxTokens:     func() *int { v := 4096; return &v }(),
		MaxIterations: 5,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// applyModelOverride
// ──────────────────────────────────────────────────────────────────────────────

func TestApplyModelOverride(t *testing.T) {
	ctx := context.Background()
	const runningUserID = "user-1"

	t.Run("agent configured for OpenAI uses OpenAI provider and model instead of user global Gemini", func(t *testing.T) {
		runner := &RunnerAdapter{apiKeyResolver: resolverWithAllProviders()}
		openAIOverride := &fagent.AgentModelOverride{Provider: "openai", ModelName: "gpt-4o"}

		resolvedConfig, err := runner.applyModelOverride(ctx, runningUserID, userGlobalGeminiConfig(), openAIOverride)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolvedConfig.Provider != "openai" {
			t.Errorf("Provider: got %q, want %q", resolvedConfig.Provider, "openai")
		}
		if resolvedConfig.Model != "gpt-4o" {
			t.Errorf("Model: got %q, want %q", resolvedConfig.Model, "gpt-4o")
		}
		if resolvedConfig.APIKey != "openai-api-key" {
			t.Errorf("APIKey: got %q, want %q", resolvedConfig.APIKey, "openai-api-key")
		}
	})

	t.Run("switching provider preserves user global temperature maxTokens and maxIterations", func(t *testing.T) {
		runner := &RunnerAdapter{apiKeyResolver: resolverWithAllProviders()}
		anthropicOverride := &fagent.AgentModelOverride{Provider: "anthropic", ModelName: "claude-3-opus-20240229"}

		resolvedConfig, err := runner.applyModelOverride(ctx, runningUserID, userGlobalGeminiConfig(), anthropicOverride)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolvedConfig.Temperature == nil || *resolvedConfig.Temperature != 0.7 {
			t.Errorf("Temperature should carry over from user global config: got %v", resolvedConfig.Temperature)
		}
		if resolvedConfig.MaxTokens == nil || *resolvedConfig.MaxTokens != 4096 {
			t.Errorf("MaxTokens should carry over from user global config: got %v", resolvedConfig.MaxTokens)
		}
		if resolvedConfig.MaxIterations != 5 {
			t.Errorf("MaxIterations should carry over from user global config: got %d", resolvedConfig.MaxIterations)
		}
	})

	t.Run("overriding model within the same provider resolves the correct API key", func(t *testing.T) {
		runner := &RunnerAdapter{apiKeyResolver: resolverWithAllProviders()}
		newerGeminiModelOverride := &fagent.AgentModelOverride{Provider: "gemini", ModelName: "gemini-1.5-pro"}

		resolvedConfig, err := runner.applyModelOverride(ctx, runningUserID, userGlobalGeminiConfig(), newerGeminiModelOverride)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolvedConfig.Model != "gemini-1.5-pro" {
			t.Errorf("Model: got %q, want %q", resolvedConfig.Model, "gemini-1.5-pro")
		}
		if resolvedConfig.Provider != "gemini" {
			t.Errorf("Provider: got %q, want %q", resolvedConfig.Provider, "gemini")
		}
	})

	t.Run("provider the user has not added an API key for returns a descriptive error", func(t *testing.T) {
		runner := &RunnerAdapter{apiKeyResolver: resolverWithAllProviders()}
		overrideForProviderWithNoKey := &fagent.AgentModelOverride{Provider: "unknown-provider", ModelName: "some-model"}

		_, err := runner.applyModelOverride(ctx, runningUserID, userGlobalGeminiConfig(), overrideForProviderWithNoKey)
		if err == nil {
			t.Error("expected error for provider with no configured API key")
		}
	})

	t.Run("runner without an API key resolver cannot apply model overrides", func(t *testing.T) {
		runnerWithNoKeyResolver := &RunnerAdapter{apiKeyResolver: nil}
		openAIOverride := &fagent.AgentModelOverride{Provider: "openai", ModelName: "gpt-4o"}

		_, err := runnerWithNoKeyResolver.applyModelOverride(ctx, runningUserID, userGlobalGeminiConfig(), openAIOverride)
		if err == nil {
			t.Error("expected error when runner has no API key resolver configured")
		}
	})

	t.Run("user global config with no generation params does not inject nil pointers into resolved config", func(t *testing.T) {
		runner := &RunnerAdapter{apiKeyResolver: resolverWithAllProviders()}
		globalConfigWithNoGenerationParams := &fagent.ModelConfig{
			Provider: "gemini",
			Model:    "gemini-2.0-flash",
			APIKey:   "gemini-api-key",
			// Temperature and MaxTokens deliberately absent — user never set them
		}
		openAIOverride := &fagent.AgentModelOverride{Provider: "openai", ModelName: "gpt-4o-mini"}

		resolvedConfig, err := runner.applyModelOverride(ctx, runningUserID, globalConfigWithNoGenerationParams, openAIOverride)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolvedConfig.Temperature != nil {
			t.Errorf("Temperature should remain nil when user global has no temperature set, got %v", resolvedConfig.Temperature)
		}
		if resolvedConfig.MaxTokens != nil {
			t.Errorf("MaxTokens should remain nil when user global has no maxTokens set, got %v", resolvedConfig.MaxTokens)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Gap 1 + Gap 2 interaction
//
// When an agent has both a model override (Gap 2) and per-agent generation
// parameter overrides (Gap 1), the model override determines which provider and
// model to use, while the Gap-1 temperature still wins over the user's global
// temperature when building the final GenerateContentConfig.
// ──────────────────────────────────────────────────────────────────────────────

func TestPerAgentModelOverride_DoesNotSuppressPerAgentGenerationConfig(t *testing.T) {
	ctx := context.Background()
	runner := &RunnerAdapter{
		apiKeyResolver: &stubAPIKeyResolver{configuredKeys: map[string]string{"openai": "openai-api-key"}},
	}

	userGlobalConfig := &fagent.ModelConfig{
		Provider:    "gemini",
		Model:       "gemini-2.0-flash",
		APIKey:      "gemini-api-key",
		Temperature: f64(0.5),
	}
	openAIOverride := &fagent.AgentModelOverride{Provider: "openai", ModelName: "gpt-4o"}

	// Apply the model override — provider/model/key swap to OpenAI, temperature carried from global.
	resolvedConfig, err := runner.applyModelOverride(ctx, "user-1", userGlobalConfig, openAIOverride)
	if err != nil {
		t.Fatalf("applyModelOverride: %v", err)
	}

	// The per-agent generation config (Gap 1) raises temperature to 1.0.
	perAgentGenerationOverrides := &fagent.AgentModelConfig{Temperature: f64(1.0)}

	generationConfig := buildGenerateContentConfig(resolvedConfig, perAgentGenerationOverrides, nil)

	// Temperature must be 1.0 from the per-agent override, not 0.5 from the user global.
	if generationConfig.Temperature == nil {
		t.Fatal("expected non-nil temperature in generation config")
	}
	if *generationConfig.Temperature != float32(1.0) {
		t.Errorf("per-agent temperature (1.0) should override user global temperature (0.5): got %f", *generationConfig.Temperature)
	}
}
