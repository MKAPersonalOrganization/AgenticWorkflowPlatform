package core

import "maps"

// RunOptions provides optional parameters for controlling agent run behavior.
// These options can override the agent's default configuration on a per-run basis.
// This mirrors the approach in Microsoft.Agents.AI.AgentRunOptions from the .NET agent framework.
type RunOptions struct {
	// Temperature controls randomness in the output (0.0-2.0).
	// When set, overrides the agent's default temperature.
	Temperature *float64

	// MaxTokens limits the maximum number of tokens to generate.
	// When set, overrides the agent's default max tokens.
	MaxTokens *int

	// TopP controls nucleus sampling (0.0-1.0).
	// When set, overrides the agent's default top-p value.
	TopP *float64

	// FrequencyPenalty penalizes tokens based on their frequency (-2.0 to 2.0).
	// When set, overrides the agent's default frequency penalty.
	FrequencyPenalty *float64

	// PresencePenalty penalizes tokens based on their presence (-2.0 to 2.0).
	// When set, overrides the agent's default presence penalty.
	PresencePenalty *float64

	// StopSequences are strings that stop generation when encountered.
	// When set, overrides the agent's default stop sequences.
	StopSequences []string

	// ModelID optionally overrides the default model for this run.
	ModelID string

	// Seed enables reproducible outputs when set.
	// When set, overrides the agent's default seed.
	Seed *int64

	// AdditionalProperties holds custom metadata or provider-specific options.
	// This can be used to pass implementation-specific details that don't fit
	// into the standard options schema.
	AdditionalProperties map[string]any
}

// Clone creates a deep copy of RunOptions.
func (o RunOptions) Clone() RunOptions {
	clone := RunOptions{
		Temperature:      o.Temperature,
		MaxTokens:        o.MaxTokens,
		TopP:             o.TopP,
		FrequencyPenalty: o.FrequencyPenalty,
		PresencePenalty:  o.PresencePenalty,
		ModelID:          o.ModelID,
		Seed:             o.Seed,
	}

	if o.StopSequences != nil {
		clone.StopSequences = make([]string, len(o.StopSequences))
		copy(clone.StopSequences, o.StopSequences)
	}

	if o.AdditionalProperties != nil {
		clone.AdditionalProperties = make(map[string]any, len(o.AdditionalProperties))
		maps.Copy(clone.AdditionalProperties, o.AdditionalProperties)
	}

	return clone
}

// DefaultRunOptions returns a RunOptions with Microsoft-compatible defaults.
// Matches the defaults from Microsoft.Agents.AI.AgentRunOptions:
// - Temperature: 1.0 (controls randomness)
// - TopP: 1.0 (nucleus sampling threshold)
// - Other parameters use zero values (nil) to defer to agent config.
func DefaultRunOptions() RunOptions {
	temperature := 1.0
	topP := 1.0

	return RunOptions{
		Temperature: &temperature,
		TopP:        &topP,
	}
}
