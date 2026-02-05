package core

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/internal/llm"
)

// EventNotifier is the interface for handling domain events from agents.
type EventNotifier interface {
	HandleDomainEvent(event Event) error
}

// Handoff represents a single handoff destination for an agent.
// This aligns with LangGraph's handoff pattern where agents declaratively
// specify which other agents they can transfer control to.
type Handoff struct {
	AgentName   string // Name of the agent to hand off to
	Description string // Description of when to use this handoff
}

// AgentConfig defines the configuration for an agent.
type AgentConfig struct {
	// Identity
	Name string // Unique identifier for the agent

	// Behavior
	SystemPrompt string                 // Instructions for how the agent should behave
	Tools        []agent.ExecutableTool // Available tools for this agent (implements ExecutableTool interface)

	// Handoffs declares which agents this agent can transfer control to.
	// This is used by HandoffMiddleware to automatically create transfer tools.
	Handoffs []Handoff // Declarative handoff configuration

	// Middleware - Separate chains for each operation type
	AgentMiddleware    []agent.Middleware // Runs around entire agent run
	ChatMiddleware     []agent.Middleware // Runs around each LLM call
	FunctionMiddleware []agent.Middleware // Runs around each tool execution

	// Execution Limits
	MaxIterations int // Maximum LLM iterations per run cycle (prevents infinite loops)

	// LLM Parameters (defaults for all runs)
	Temperature      *float64 // Default temperature (0.0-2.0)
	MaxTokens        *int     // Default max tokens to generate
	TopP             *float64 // Default top-p value (0.0-1.0)
	FrequencyPenalty *float64 // Default frequency penalty (-2.0 to 2.0)
	PresencePenalty  *float64 // Default presence penalty (-2.0 to 2.0)
	StopSequences    []string // Default stop sequences
	ModelID          string   // Default model ID
	Provider         string   // LLM provider name (e.g., "openai", "gemini")
	Seed             *int64   // Default seed for reproducibility

	// Event handling
	Notifier EventNotifier // UI/Event notifier for this agent
}

// Validate ensures the agent configuration is valid.
func (c *AgentConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if c.MaxIterations <= 0 {
		return fmt.Errorf("agent %q: MaxIterations must be greater than 0", c.Name)
	}
	return nil
}

// DefaultConfig returns a basic agent configuration.
func DefaultConfig(name string) AgentConfig {
	return AgentConfig{
		Name:               name,
		SystemPrompt:       "You are a helpful assistant.",
		Tools:              []agent.ExecutableTool{},
		AgentMiddleware:    []agent.Middleware{},
		ChatMiddleware:     []agent.Middleware{},
		FunctionMiddleware: []agent.Middleware{},
		MaxIterations:      5,
	}
}

// ToToolDeclarations converts agent.ExecutableTool slice to llm.ToolDeclaration slice for ChatClient.
func ToToolDeclarations(tools []agent.ExecutableTool) []llm.ToolDeclaration {
	result := make([]llm.ToolDeclaration, len(tools))
	for i, t := range tools {
		// Convert agent.Schema to llm.Schema
		schema := t.Schema()
		var llmSchema *llm.Schema
		if schema != nil {
			// Convert agent.Schema to llm.Schema via JSON
			data, _ := json.Marshal(schema)
			_ = json.Unmarshal(data, &llmSchema)
		}

		result[i] = llm.ToolDeclaration{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  llmSchema,
		}
	}

	return result
}

// ToLLMRequestWithMiddleware builds llm.Request from agent configuration and run options,
// applying middleware transformations. This is the middleware-aware version of building a request.
// It executes the middleware chain to collect additional tools and system prompt modifications.
// Returns the llm.Request and the RequestContext (which may have Terminate=true if middleware
// short-circuited).
func ToLLMRequestWithMiddleware(ctx context.Context, config AgentConfig, messages []llm.Message, threadID string, runOpts RunOptions) (*llm.Request, *RequestContext, error) {
	// Apply middleware to get the request context
	reqCtx, err := ApplyMiddleware(ctx, config, messages, threadID, runOpts)
	if err != nil {
		return nil, nil, err
	}

	// Build Parameters
	params := &llm.Parameters{}

	// Apply config parameters from request context
	if reqCtx.Config.Temperature != nil {
		params.Temperature = reqCtx.Config.Temperature
	}
	if reqCtx.Config.MaxTokens != nil {
		params.MaxTokens = *reqCtx.Config.MaxTokens
	}
	if reqCtx.Config.TopP != nil {
		params.TopP = reqCtx.Config.TopP
	}
	if reqCtx.Config.FrequencyPenalty != nil {
		params.FrequencyPenalty = reqCtx.Config.FrequencyPenalty
	}
	if reqCtx.Config.PresencePenalty != nil {
		params.PresencePenalty = reqCtx.Config.PresencePenalty
	}
	if reqCtx.Config.StopSequences != nil {
		params.StopSequences = make([]string, len(reqCtx.Config.StopSequences))
		copy(params.StopSequences, reqCtx.Config.StopSequences)
	}

	// Extensions
	extensions := llm.Metadata{}
	if reqCtx.Config.Provider != "" {
		extensions["provider"] = reqCtx.Config.Provider
	}
	if reqCtx.Config.ModelID != "" {
		extensions["model"] = reqCtx.Config.ModelID
	}
	if reqCtx.Config.Seed != nil {
		extensions["seed"] = *reqCtx.Config.Seed
	}

	// Merge additional properties from run options
	if len(runOpts.AdditionalProperties) > 0 {
		maps.Copy(extensions, runOpts.AdditionalProperties)
	}

	// Build llm.Request
	req := &llm.Request{
		Messages:   messages,
		System:     reqCtx.GetEffectiveSystemPrompt(),
		Tools:      ToToolDeclarations(reqCtx.GetEffectiveTools()),
		Parameters: params,
		Extensions: extensions,
	}

	return req, reqCtx, nil
}

// ApplyMiddleware executes the middleware chain on initial request context
// to collect additional tools and system prompt modifications. This is called
// before the LLM request to gather middleware contributions.
func ApplyMiddleware(ctx context.Context, config AgentConfig, messages []llm.Message, threadID string, runOpts RunOptions) (*RequestContext, error) {
	// Build initial request context with minimal config
	initialConfig := &AgentConfig{}
	reqCtx := NewRequestContext(threadID, messages, config.SystemPrompt, config.Tools, initialConfig)

	// Copy essential config fields to request context
	reqCtx.Config.Name = config.Name
	reqCtx.Config.Handoffs = config.Handoffs

	// Copy config parameters to request context
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
	if config.Provider != "" {
		reqCtx.Config.Provider = config.Provider
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

	// Execute agent middleware chain if any middleware is configured
	if len(config.AgentMiddleware) > 0 {
		// Create a chat context for the middleware chain
		// (This middleware runs before the LLM call)
		chatCtx := agent.NewChatContext(ctx, nil, nil, nil)
		chatCtx.DryRun = true

		// Store request context in metadata for middleware access
		chatCtx.Metadata["request_context"] = reqCtx

		// Build the handler chain
		finalHandler := agent.HandlerFunc(func(c *agent.Context) error {
			return nil // No-op - we're only running the "before" phase here
		})

		chain := agent.Chain(config.AgentMiddleware...)
		err := chain(finalHandler).Handle(chatCtx)
		if err != nil {
			return nil, err
		}

		// Check for termination signal in metadata
		if terminate, ok := chatCtx.Metadata["terminate"].(bool); ok && terminate {
			reqCtx.Terminate = true
			return reqCtx, nil
		}
	}

	return reqCtx, nil
}
