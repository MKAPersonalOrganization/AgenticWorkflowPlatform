package core

import (
	"context"
	"maps"
	"school-with-ai/foundation/agentsdk/llm"
)

// EventNotifier is the interface for handling domain events from agents.
type EventNotifier interface {
	HandleDomainEvent(event Event) error
}

// AgentConfig defines the configuration for an agent.
type AgentConfig struct {
	// Identity
	Name string // Unique identifier for the agent
	// SubAgents are child agents that this agent can delegate tasks to as tools.
	// This enables the "Centralized Orchestration" pattern.
	SubAgents []AgentConfig

	// HandoffAgents are peer agents that this agent can transfer control to.
	// This enables the "State-driven Transitions" (Handoffs) pattern.
	HandoffAgents []AgentConfig

	// Behavior
	SystemPrompt string // Instructions for how the agent should behave
	Tools        []Tool // Available tools for this agent (implements Tool interface)

	// Middleware
	Middleware []Middleware // Middleware chain for request/response processing

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

	// Execution delegation
	Runner SubagentRunner // For centralized orchestration of subagents
}

// SubagentRunner defines the interface for running specialized subagents.
// This is typically implemented by the service or engine to avoid circular dependencies.
type SubagentRunner interface {
	RunSubagent(ctx context.Context, agent AgentConfig, threadID string, question string) (string, error)
}

// DefaultConfig returns a basic agent configuration.
func DefaultConfig(name string) AgentConfig {
	return AgentConfig{
		Name:         name,
		SystemPrompt: "You are a helpful assistant.",
		Tools:        []Tool{},
		Middleware:   []Middleware{},
	}
}

// ToAITools converts agent.Tool slice to llm.AITool slice for ChatClient.
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

// ToChatOptionsWithMiddleware builds ChatOptions from agent configuration and run options,
// applying middleware transformations. This is the middleware-aware version of ToChatOptions.
// It executes the middleware chain to collect additional tools and system prompt modifications.
// Returns the ChatOptions and the RequestContext (which may have Terminate=true if middleware
// short-circuited).
func ToChatOptionsWithMiddleware(ctx context.Context, config AgentConfig, messages []llm.ChatMessage, threadID string, runOpts RunOptions) (*llm.ChatOptions, *RequestContext, error) {
	// Apply middleware to get the request context
	reqCtx, err := ApplyMiddleware(ctx, config, messages, threadID, runOpts)
	if err != nil {
		return nil, nil, err
	}

	// Build ChatOptions from the middleware-modified request context
	opts := &llm.ChatOptions{
		Instructions: reqCtx.GetEffectiveSystemPrompt(),
		Tools:        ToAITools(reqCtx.GetEffectiveTools()),
		ToolMode:     llm.ToolModeAuto,
	}

	// Apply config parameters from request context
	if reqCtx.Config.Temperature != nil {
		opts.Temperature = reqCtx.Config.Temperature
	}
	if reqCtx.Config.MaxTokens != nil {
		opts.MaxOutputTokens = reqCtx.Config.MaxTokens
	}
	if reqCtx.Config.TopP != nil {
		opts.TopP = reqCtx.Config.TopP
	}
	if reqCtx.Config.FrequencyPenalty != nil {
		opts.FrequencyPenalty = reqCtx.Config.FrequencyPenalty
	}
	if reqCtx.Config.PresencePenalty != nil {
		opts.PresencePenalty = reqCtx.Config.PresencePenalty
	}
	if reqCtx.Config.StopSequences != nil {
		opts.StopSequences = make([]string, len(reqCtx.Config.StopSequences))
		copy(opts.StopSequences, reqCtx.Config.StopSequences)
	}
	if reqCtx.Config.ModelID != "" {
		opts.ModelID = reqCtx.Config.ModelID
	}
	if reqCtx.Config.Seed != nil {
		opts.Seed = reqCtx.Config.Seed
	}

	// Merge additional properties from run options
	if len(runOpts.AdditionalProperties) > 0 {
		if opts.AdditionalProperties == nil {
			opts.AdditionalProperties = make(map[string]any)
		}
		maps.Copy(opts.AdditionalProperties, runOpts.AdditionalProperties)
	}

	return opts, reqCtx, nil
}

// ApplyMiddleware executes the middleware chain on initial request context
// to collect additional tools and system prompt modifications. This is called
// before the LLM request to gather middleware contributions.
func ApplyMiddleware(ctx context.Context, config AgentConfig, messages []llm.ChatMessage, threadID string, runOpts RunOptions) (*RequestContext, error) {
	// Build initial request context with the base configuration
	reqCtxConfig := &AgentConfig{
		Name:          config.Name,
		SubAgents:     config.SubAgents,
		HandoffAgents: config.HandoffAgents,
		Runner:        config.Runner,
		Notifier:      config.Notifier,
	}
	reqCtx := NewRequestContext(threadID, messages, config.SystemPrompt, config.Tools, reqCtxConfig)

	// Copy LLM config parameters to request context
	if config.Temperature != nil {
		reqCtx.Config.Temperature = config.Temperature
	}
	if config.MaxTokens != nil {
		reqCtx.Config.MaxTokens = config.MaxTokens
	}
	if config.TopP != nil {
		reqCtx.Config.TopP = config.TopP
	}
	if config.FrequencyPenalty != nil {
		reqCtx.Config.FrequencyPenalty = config.FrequencyPenalty
	}
	if config.PresencePenalty != nil {
		reqCtx.Config.PresencePenalty = config.PresencePenalty
	}
	if config.StopSequences != nil {
		reqCtx.Config.StopSequences = make([]string, len(config.StopSequences))
		copy(reqCtx.Config.StopSequences, config.StopSequences)
	}
	if config.ModelID != "" {
		reqCtx.Config.ModelID = config.ModelID
	}
	if config.Seed != nil {
		reqCtx.Config.Seed = config.Seed
	}

	// Apply run options overrides
	if runOpts.Temperature != nil {
		reqCtx.Config.Temperature = runOpts.Temperature
	}
	if runOpts.MaxTokens != nil {
		reqCtx.Config.MaxTokens = runOpts.MaxTokens
	}
	if runOpts.TopP != nil {
		reqCtx.Config.TopP = runOpts.TopP
	}
	if runOpts.FrequencyPenalty != nil {
		reqCtx.Config.FrequencyPenalty = runOpts.FrequencyPenalty
	}
	if runOpts.PresencePenalty != nil {
		reqCtx.Config.PresencePenalty = runOpts.PresencePenalty
	}
	if runOpts.StopSequences != nil {
		reqCtx.Config.StopSequences = make([]string, len(runOpts.StopSequences))
		copy(reqCtx.Config.StopSequences, runOpts.StopSequences)
	}
	if runOpts.ModelID != "" {
		reqCtx.Config.ModelID = runOpts.ModelID
	}
	if runOpts.Seed != nil {
		reqCtx.Config.Seed = runOpts.Seed
	}
	// Note: AdditionalProperties are handled separately in ChatOptions

	// Execute middleware chain if any middleware is configured
	if len(config.Middleware) > 0 {
		// Use a no-op handler - we're only running the "before" phase here
		// The actual LLM call happens in the service layer
		noOpHandler := func(ctx context.Context, reqCtx *RequestContext) error {
			return nil // No-op
		}

		chain := ChainMiddleware(config.Middleware, noOpHandler)

		err := chain(ctx, reqCtx)
		if err != nil {
			return nil, err
		}

		// Check if middleware terminated early
		if reqCtx.Terminate {
			return reqCtx, nil
		}
	}

	return reqCtx, nil
}
