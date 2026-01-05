package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOptionsClone(t *testing.T) {
	temp := 0.7
	maxTokens := 100
	topP := 0.9
	freqPenalty := 0.5
	presPenalty := 0.3
	seed := int64(42)

	original := RunOptions{
		Temperature:      &temp,
		MaxTokens:        &maxTokens,
		TopP:             &topP,
		FrequencyPenalty: &freqPenalty,
		PresencePenalty:  &presPenalty,
		StopSequences:    []string{"stop1", "stop2"},
		ModelID:          "gpt-4",
		Seed:             &seed,
		AdditionalProperties: map[string]interface{}{
			"custom": "value",
		},
	}

	clone := original.Clone()

	// Verify values are equal
	assert.Equal(t, *original.Temperature, *clone.Temperature)
	assert.Equal(t, *original.MaxTokens, *clone.MaxTokens)
	assert.Equal(t, *original.TopP, *clone.TopP)
	assert.Equal(t, *original.FrequencyPenalty, *clone.FrequencyPenalty)
	assert.Equal(t, *original.PresencePenalty, *clone.PresencePenalty)
	assert.Equal(t, original.StopSequences, clone.StopSequences)
	assert.Equal(t, original.ModelID, clone.ModelID)
	assert.Equal(t, *original.Seed, *clone.Seed)
	assert.Equal(t, original.AdditionalProperties, clone.AdditionalProperties)

	// Verify deep copy (mutations don't affect original)
	clone.StopSequences[0] = "modified"
	assert.NotEqual(t, original.StopSequences[0], clone.StopSequences[0])

	clone.AdditionalProperties["custom"] = "modified"
	assert.NotEqual(t, original.AdditionalProperties["custom"], clone.AdditionalProperties["custom"])
}

func TestDefaultRunOptions(t *testing.T) {
	opts := DefaultRunOptions()

	// Microsoft framework defaults: Temperature=1.0, TopP=1.0
	require.NotNil(t, opts.Temperature)
	assert.Equal(t, 1.0, *opts.Temperature)
	require.NotNil(t, opts.TopP)
	assert.Equal(t, 1.0, *opts.TopP)

	// Other fields remain nil (defer to agent config)
	assert.Nil(t, opts.MaxTokens)
	assert.Nil(t, opts.FrequencyPenalty)
	assert.Nil(t, opts.PresencePenalty)
	assert.Nil(t, opts.StopSequences)
	assert.Empty(t, opts.ModelID)
	assert.Nil(t, opts.Seed)
	assert.Nil(t, opts.AdditionalProperties)
}

func TestToChatOptionsWithDefaultRunOptions(t *testing.T) {
	temp := 0.5
	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "Test prompt",
		Temperature:  &temp,
	}

	chatOpts := ToChatOptions(config, DefaultRunOptions())

	require.NotNil(t, chatOpts)
	assert.Equal(t, "Test prompt", chatOpts.Instructions)
	// DefaultRunOptions has Temperature=1.0, which overrides config's 0.5
	require.NotNil(t, chatOpts.Temperature)
	assert.Equal(t, 1.0, *chatOpts.Temperature)
}

func TestToChatOptionsRunOptionsOverridesConfig(t *testing.T) {
	configTemp := 0.5
	configMaxTokens := 100
	configTopP := 0.8

	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "Test prompt",
		Temperature:  &configTemp,
		MaxTokens:    &configMaxTokens,
		TopP:         &configTopP,
		ModelID:      "gpt-3.5",
	}

	runTemp := 0.9
	runMaxTokens := 200
	runOpts := RunOptions{
		Temperature: &runTemp,
		MaxTokens:   &runMaxTokens,
		ModelID:     "gpt-4",
	}

	chatOpts := ToChatOptions(config, runOpts)

	require.NotNil(t, chatOpts)

	// RunOptions should override config
	require.NotNil(t, chatOpts.Temperature)
	assert.Equal(t, 0.9, *chatOpts.Temperature, "Temperature should be overridden by RunOptions")

	require.NotNil(t, chatOpts.MaxOutputTokens)
	assert.Equal(t, 200, *chatOpts.MaxOutputTokens, "MaxTokens should be overridden by RunOptions")

	assert.Equal(t, "gpt-4", chatOpts.ModelID, "ModelID should be overridden by RunOptions")

	// TopP should come from config (not in RunOptions)
	require.NotNil(t, chatOpts.TopP)
	assert.Equal(t, 0.8, *chatOpts.TopP, "TopP should come from config")
}

func TestToChatOptionsPartialOverride(t *testing.T) {
	configTemp := 0.5
	configMaxTokens := 100
	configTopP := 0.8
	configFreqPenalty := 0.2
	configSeed := int64(123)

	config := AgentConfig{
		Name:             "test-agent",
		SystemPrompt:     "Test prompt",
		Temperature:      &configTemp,
		MaxTokens:        &configMaxTokens,
		TopP:             &configTopP,
		FrequencyPenalty: &configFreqPenalty,
		StopSequences:    []string{"config-stop"},
		ModelID:          "gpt-3.5",
		Seed:             &configSeed,
	}

	runTemp := 0.9
	runOpts := RunOptions{
		Temperature:   &runTemp,
		StopSequences: []string{"run-stop-1", "run-stop-2"},
	}

	chatOpts := ToChatOptions(config, runOpts)

	require.NotNil(t, chatOpts)

	// Overridden values
	require.NotNil(t, chatOpts.Temperature)
	assert.Equal(t, 0.9, *chatOpts.Temperature)
	assert.Equal(t, []string{"run-stop-1", "run-stop-2"}, chatOpts.StopSequences)

	// Config values preserved
	require.NotNil(t, chatOpts.MaxOutputTokens)
	assert.Equal(t, 100, *chatOpts.MaxOutputTokens)
	require.NotNil(t, chatOpts.TopP)
	assert.Equal(t, 0.8, *chatOpts.TopP)
	require.NotNil(t, chatOpts.FrequencyPenalty)
	assert.Equal(t, 0.2, *chatOpts.FrequencyPenalty)
	assert.Equal(t, "gpt-3.5", chatOpts.ModelID)
	require.NotNil(t, chatOpts.Seed)
	assert.Equal(t, int64(123), *chatOpts.Seed)
}

func TestToChatOptionsAdditionalProperties(t *testing.T) {
	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "Test prompt",
	}

	runOpts := RunOptions{
		AdditionalProperties: map[string]interface{}{
			"customKey1": "value1",
			"customKey2": 42,
			"customKey3": true,
		},
	}

	chatOpts := ToChatOptions(config, runOpts)

	require.NotNil(t, chatOpts)
	require.NotNil(t, chatOpts.AdditionalProperties)
	assert.Equal(t, "value1", chatOpts.AdditionalProperties["customKey1"])
	assert.Equal(t, 42, chatOpts.AdditionalProperties["customKey2"])
	assert.Equal(t, true, chatOpts.AdditionalProperties["customKey3"])
}

func TestToChatOptionsAllParameters(t *testing.T) {
	configTemp := 0.5
	configMaxTokens := 100
	configTopP := 0.8
	configFreqPenalty := 0.2
	configPresPenalty := 0.1
	configSeed := int64(123)

	config := AgentConfig{
		Name:             "test-agent",
		SystemPrompt:     "Test prompt",
		Temperature:      &configTemp,
		MaxTokens:        &configMaxTokens,
		TopP:             &configTopP,
		FrequencyPenalty: &configFreqPenalty,
		PresencePenalty:  &configPresPenalty,
		StopSequences:    []string{"config-stop"},
		ModelID:          "gpt-3.5",
		Seed:             &configSeed,
	}

	runTemp := 0.9
	runMaxTokens := 200
	runTopP := 0.95
	runFreqPenalty := 0.5
	runPresPenalty := 0.3
	runSeed := int64(456)

	runOpts := RunOptions{
		Temperature:      &runTemp,
		MaxTokens:        &runMaxTokens,
		TopP:             &runTopP,
		FrequencyPenalty: &runFreqPenalty,
		PresencePenalty:  &runPresPenalty,
		StopSequences:    []string{"run-stop-1", "run-stop-2"},
		ModelID:          "gpt-4",
		Seed:             &runSeed,
		AdditionalProperties: map[string]interface{}{
			"custom": "value",
		},
	}

	chatOpts := ToChatOptions(config, runOpts)

	require.NotNil(t, chatOpts)

	// All values should come from runOpts
	require.NotNil(t, chatOpts.Temperature)
	assert.Equal(t, 0.9, *chatOpts.Temperature)
	require.NotNil(t, chatOpts.MaxOutputTokens)
	assert.Equal(t, 200, *chatOpts.MaxOutputTokens)
	require.NotNil(t, chatOpts.TopP)
	assert.Equal(t, 0.95, *chatOpts.TopP)
	require.NotNil(t, chatOpts.FrequencyPenalty)
	assert.Equal(t, 0.5, *chatOpts.FrequencyPenalty)
	require.NotNil(t, chatOpts.PresencePenalty)
	assert.Equal(t, 0.3, *chatOpts.PresencePenalty)
	assert.Equal(t, []string{"run-stop-1", "run-stop-2"}, chatOpts.StopSequences)
	assert.Equal(t, "gpt-4", chatOpts.ModelID)
	require.NotNil(t, chatOpts.Seed)
	assert.Equal(t, int64(456), *chatOpts.Seed)
	require.NotNil(t, chatOpts.AdditionalProperties)
	assert.Equal(t, "value", chatOpts.AdditionalProperties["custom"])
}
