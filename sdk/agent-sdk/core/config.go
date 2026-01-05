package sdk

import "school-with-ai/sdk/agent-sdk/llm"

// EventNotifier is the interface for handling domain events from agents
type EventNotifier interface {
	HandleDomainEvent(event Event) error
}

// AgentConfig defines the configuration for an agent
type AgentConfig struct {
	// Identity
	Name string // Unique identifier for the agent

	// Behavior
	SystemPrompt string // Instructions for how the agent should behave
	Tools        []Tool // Available tools for this agent (implements Tool interface)

	// LLM Parameters (defaults for all runs)
	Temperature      *float64 // Default temperature (0.0-2.0)
	MaxTokens        *int     // Default max tokens to generate
	TopP             *float64 // Default top-p value (0.0-1.0)
	FrequencyPenalty *float64 // Default frequency penalty (-2.0 to 2.0)
	PresencePenalty  *float64 // Default presence penalty (-2.0 to 2.0)
	StopSequences    []string // Default stop sequences
	ModelID          string   // Default model ID
	Seed             *int64   // Default seed for reproducibility

	// Event handling
	Notifier EventNotifier // UI/Event notifier for this agent
}

// DefaultConfig returns a basic agent configuration
func DefaultConfig(name string) AgentConfig {
	return AgentConfig{
		Name:         name,
		SystemPrompt: "You are a helpful assistant.",
		Tools:        []Tool{},
	}
}

// ToAITools converts agent.Tool slice to llm.AITool slice for ChatClient
func ToAITools(tools []Tool) []llm.AITool {
	result := make([]llm.AITool, len(tools))
	for i, t := range tools {
		result[i] = llm.NewAIFunctionTool(
			t.Name(),
			t.Description(),
			t.Parameters(),
		)
	}
	return result
}

// ToChatOptions builds ChatOptions from agent configuration and run options.
// RunOptions take precedence over AgentConfig values when both are set.
func ToChatOptions(config AgentConfig, runOpts RunOptions) *llm.ChatOptions {
	opts := &llm.ChatOptions{
		Instructions: config.SystemPrompt,
		Tools:        ToAITools(config.Tools),
		ToolMode:     llm.ToolModeAuto,
	}

	// Apply config defaults
	if config.Temperature != nil {
		opts.Temperature = config.Temperature
	}
	if config.MaxTokens != nil {
		opts.MaxOutputTokens = config.MaxTokens
	}
	if config.TopP != nil {
		opts.TopP = config.TopP
	}
	if config.FrequencyPenalty != nil {
		opts.FrequencyPenalty = config.FrequencyPenalty
	}
	if config.PresencePenalty != nil {
		opts.PresencePenalty = config.PresencePenalty
	}
	if config.StopSequences != nil {
		opts.StopSequences = make([]string, len(config.StopSequences))
		copy(opts.StopSequences, config.StopSequences)
	}
	if config.ModelID != "" {
		opts.ModelID = config.ModelID
	}
	if config.Seed != nil {
		opts.Seed = config.Seed
	}

	// Apply run options overrides
	if runOpts.Temperature != nil {
		opts.Temperature = runOpts.Temperature
	}
	if runOpts.MaxTokens != nil {
		opts.MaxOutputTokens = runOpts.MaxTokens
	}
	if runOpts.TopP != nil {
		opts.TopP = runOpts.TopP
	}
	if runOpts.FrequencyPenalty != nil {
		opts.FrequencyPenalty = runOpts.FrequencyPenalty
	}
	if runOpts.PresencePenalty != nil {
		opts.PresencePenalty = runOpts.PresencePenalty
	}
	if runOpts.StopSequences != nil {
		opts.StopSequences = make([]string, len(runOpts.StopSequences))
		copy(opts.StopSequences, runOpts.StopSequences)
	}
	if runOpts.ModelID != "" {
		opts.ModelID = runOpts.ModelID
	}
	if runOpts.Seed != nil {
		opts.Seed = runOpts.Seed
	}
	// Merge additional properties
	if runOpts.AdditionalProperties != nil {
		if opts.AdditionalProperties == nil {
			opts.AdditionalProperties = make(map[string]any)
		}
		for k, v := range runOpts.AdditionalProperties {
			opts.AdditionalProperties[k] = v
		}
	}

	return opts
}
