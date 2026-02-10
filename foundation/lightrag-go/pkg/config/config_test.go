package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 9621, cfg.Port)
	assert.Equal(t, 2, cfg.Workers)
	assert.Equal(t, "./rag_storage", cfg.WorkingDir)
	assert.Equal(t, "./inputs", cfg.InputDir)
	assert.Equal(t, "ollama", cfg.LLMBinding)
	assert.Equal(t, "ollama", cfg.EmbeddingBinding)
	assert.Equal(t, "null", cfg.RerankBinding)
	assert.Equal(t, 40, cfg.TopK)
	assert.Equal(t, 20, cfg.ChunkTopK)
	assert.Equal(t, 6000, cfg.MaxEntityTokens)
	assert.Equal(t, 8000, cfg.MaxRelationTokens)
	assert.Equal(t, 30000, cfg.MaxTotalTokens)
	assert.Equal(t, 0.2, cfg.CosineThreshold)
	assert.Equal(t, true, cfg.EnableLLMCache)
	assert.Equal(t, true, cfg.EnableLLMCacheForExtract)
	assert.Equal(t, "lightrag", cfg.SimulatedModelName)
	assert.Equal(t, "latest", cfg.SimulatedModelTag)
	assert.Equal(t, "*", cfg.CORSOrigins)
	assert.Equal(t, 48.0, cfg.TokenExpireHours)
	assert.Equal(t, 24.0, cfg.GuestTokenExpireHours)
	assert.Equal(t, true, cfg.TokenAutoRenew)
	assert.Equal(t, 0.5, cfg.TokenRenewThreshold)
}

func TestLoadFromEnv(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"HOST":               os.Getenv("HOST"),
		"PORT":               os.Getenv("PORT"),
		"WORKING_DIR":        os.Getenv("WORKING_DIR"),
		"LLM_BINDING":        os.Getenv("LLM_BINDING"),
		"TOP_K":              os.Getenv("TOP_K"),
		"COSINE_THRESHOLD":   os.Getenv("COSINE_THRESHOLD"),
		"ENABLE_LLM_CACHE":   os.Getenv("ENABLE_LLM_CACHE"),
		"LIGHTRAG_API_KEY":   os.Getenv("LIGHTRAG_API_KEY"),
		"TOKEN_EXPIRE_HOURS": os.Getenv("TOKEN_EXPIRE_HOURS"),
	}

	// Restore original env vars after test
	defer func() {
		for k, v := range originalVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Set test env vars
	os.Setenv("HOST", "127.0.0.1")
	os.Setenv("PORT", "8080")
	os.Setenv("WORKING_DIR", "/custom/rag")
	os.Setenv("LLM_BINDING", "openai")
	os.Setenv("TOP_K", "100")
	os.Setenv("COSINE_THRESHOLD", "0.5")
	os.Setenv("ENABLE_LLM_CACHE", "false")
	os.Setenv("LIGHTRAG_API_KEY", "test-api-key")
	os.Setenv("TOKEN_EXPIRE_HOURS", "72")

	cfg := DefaultConfig()
	cfg.LoadFromEnv()

	assert.Equal(t, "127.0.0.1", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "/custom/rag", cfg.WorkingDir)
	assert.Equal(t, "openai", cfg.LLMBinding)
	assert.Equal(t, 100, cfg.TopK)
	assert.Equal(t, 0.5, cfg.CosineThreshold)
	assert.Equal(t, false, cfg.EnableLLMCache)
	assert.Equal(t, "test-api-key", cfg.APIKey)
	assert.Equal(t, 72.0, cfg.TokenExpireHours)
}

func TestGetDefaultHost(t *testing.T) {
	tests := []struct {
		binding  string
		expected string
	}{
		{"ollama", "http://localhost:11434"},
		{"lollms", "http://localhost:9600"},
		{"openai", "https://api.openai.com/v1"},
		{"azure_openai", "https://api.openai.com/v1"},
		{"gemini", "https://generativelanguage.googleapis.com"},
		{"unknown", "http://localhost:11434"},
	}

	for _, tt := range tests {
		t.Run(tt.binding, func(t *testing.T) {
			host := getDefaultHost(tt.binding)
			assert.Equal(t, tt.expected, host)
		})
	}
}

func TestParseEntityTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple comma separated",
			input:    "Person,Organization,Location",
			expected: []string{"Person", "Organization", "Location"},
		},
		{
			name:     "with brackets",
			input:    "[Person,Organization,Location]",
			expected: []string{"Person", "Organization", "Location"},
		},
		{
			name:     "with quotes",
			input:    `["Person","Organization","Location"]`,
			expected: []string{"Person", "Organization", "Location"},
		},
		{
			name:     "with spaces",
			input:    " Person , Organization , Location ",
			expected: []string{"Person", "Organization", "Location"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseEntityTypes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOllamaServerInfos(t *testing.T) {
	t.Run("with custom values", func(t *testing.T) {
		info := NewOllamaServerInfos("custom-model", "v1.0")

		assert.Equal(t, "custom-model", info.LightRAGName)
		assert.Equal(t, "v1.0", info.LightRAGTag)
		assert.Equal(t, "custom-model:v1.0", info.LightRAGModel())
	})

	t.Run("with defaults", func(t *testing.T) {
		// Clear env vars
		os.Unsetenv("OLLAMA_EMULATING_MODEL_NAME")
		os.Unsetenv("OLLAMA_EMULATING_MODEL_TAG")

		info := NewOllamaServerInfos("", "")

		assert.Equal(t, "lightrag", info.LightRAGName)
		assert.Equal(t, "latest", info.LightRAGTag)
		assert.Equal(t, "lightrag:latest", info.LightRAGModel())
	})
}

func TestGetEnvHelpers(t *testing.T) {
	// Save and restore original env
	origKey := os.Getenv("TEST_KEY")
	defer func() {
		if origKey == "" {
			os.Unsetenv("TEST_KEY")
		} else {
			os.Setenv("TEST_KEY", origKey)
		}
	}()

	t.Run("getEnvString", func(t *testing.T) {
		os.Unsetenv("TEST_KEY")
		assert.Equal(t, "default", getEnvString("TEST_KEY", "default"))

		os.Setenv("TEST_KEY", "value")
		assert.Equal(t, "value", getEnvString("TEST_KEY", "default"))
	})

	t.Run("getEnvInt", func(t *testing.T) {
		os.Unsetenv("TEST_KEY")
		assert.Equal(t, 42, getEnvInt("TEST_KEY", 42))

		os.Setenv("TEST_KEY", "100")
		assert.Equal(t, 100, getEnvInt("TEST_KEY", 42))

		os.Setenv("TEST_KEY", "invalid")
		assert.Equal(t, 42, getEnvInt("TEST_KEY", 42))
	})

	t.Run("getEnvFloat", func(t *testing.T) {
		os.Unsetenv("TEST_KEY")
		assert.Equal(t, 3.14, getEnvFloat("TEST_KEY", 3.14))

		os.Setenv("TEST_KEY", "2.71")
		assert.Equal(t, 2.71, getEnvFloat("TEST_KEY", 3.14))

		os.Setenv("TEST_KEY", "invalid")
		assert.Equal(t, 3.14, getEnvFloat("TEST_KEY", 3.14))
	})

	t.Run("getEnvBool", func(t *testing.T) {
		os.Unsetenv("TEST_KEY")
		assert.Equal(t, true, getEnvBool("TEST_KEY", true))
		assert.Equal(t, false, getEnvBool("TEST_KEY", false))

		os.Setenv("TEST_KEY", "true")
		assert.Equal(t, true, getEnvBool("TEST_KEY", false))

		os.Setenv("TEST_KEY", "True")
		assert.Equal(t, true, getEnvBool("TEST_KEY", false))

		os.Setenv("TEST_KEY", "1")
		assert.Equal(t, true, getEnvBool("TEST_KEY", false))

		os.Setenv("TEST_KEY", "false")
		assert.Equal(t, false, getEnvBool("TEST_KEY", true))
	})
}

func TestGlobalConfig(t *testing.T) {
	Init()
	require.NotNil(t, GlobalConfig)
	assert.Equal(t, "0.0.0.0", GlobalConfig.Host)
}
