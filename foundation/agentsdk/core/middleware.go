package core

import (
	"errors"
	"fmt"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/internal/llm"
	"strings"
)

var (
	ErrNoEventNotifier     = errors.New("no event notifier configured")
	ErrEventEmissionFailed = errors.New("failed to handle domain event")
)

// RequestContext carries all state through the middleware chain.
// It contains the request data, agent configuration, and execution state.
type RequestContext struct {
	// ThreadID is the identifier for the conversation thread
	ThreadID string

	// Messages are the conversation messages being sent to the LLM
	Messages []llm.Message

	// SystemPrompt is the system-level instruction for the agent
	SystemPrompt string

	// Tools are the available tools/functions for this request
	Tools []agent.ExecutableTool

	// Config is the agent configuration
	Config *AgentConfig

	// State is a key-value store for sharing data between middleware
	// Middleware can use this to store and retrieve arbitrary data
	State map[string]any

	// Result contains the execution result after the base handler runs
	// For LLM calls, this will be the chat response
	Result any

	// Terminate indicates whether to stop execution after current middleware
	// When true, the middleware chain will not continue
	Terminate bool

	// AdditionalTools are tools added by middleware
	// These will be merged with the base Tools
	AdditionalTools []agent.ExecutableTool

	// SystemPromptAdditions are system prompt fragments added by middleware
	// These will be appended to the base SystemPrompt
	SystemPromptAdditions []string
}

// NewRequestContext creates a new RequestContext with the given parameters.
func NewRequestContext(threadID string, messages []llm.Message, systemPrompt string, tools []agent.ExecutableTool, config *AgentConfig) *RequestContext {
	return &RequestContext{
		ThreadID:              threadID,
		Messages:              messages,
		SystemPrompt:          systemPrompt,
		Tools:                 tools,
		Config:                config,
		State:                 make(map[string]any),
		AdditionalTools:       []agent.ExecutableTool{},
		SystemPromptAdditions: []string{},
		Terminate:             false,
	}
}

// WithMetadata adds a key-value pair to the context state.
// This is useful for middleware to share information.
func (rc *RequestContext) WithMetadata(key string, value any) *RequestContext {
	rc.State[key] = value

	return rc
}

// GetMetadata retrieves a value from the context state.
// Returns the value and a boolean indicating if the key exists.
func (rc *RequestContext) GetMetadata(key string) (any, bool) {
	val, ok := rc.State[key]

	return val, ok
}

// SetTerminate marks the context for termination.
// When called, the middleware chain will stop after the current middleware completes.
func (rc *RequestContext) SetTerminate() {
	rc.Terminate = true
}

// AddTool adds a tool that will be available for this request.
// This allows middleware to dynamically inject tools.
func (rc *RequestContext) AddTool(tool agent.ExecutableTool) {
	rc.AdditionalTools = append(rc.AdditionalTools, tool)
}

// AddSystemPrompt appends additional text to the system prompt.
// This allows middleware to inject instructions without replacing the base prompt.
func (rc *RequestContext) AddSystemPrompt(text string) {
	if text != "" {
		rc.SystemPromptAdditions = append(rc.SystemPromptAdditions, text)
	}
}

// EmitEvent emits a domain event using the configured notifier.
// This is a convenience method for tools and middleware to emit events.
// Returns an error if no notifier is configured or if the event emission fails.
func (rc *RequestContext) EmitEvent(event Event) error {
	if rc.Config == nil || rc.Config.Notifier == nil {
		return ErrNoEventNotifier
	}

	err := rc.Config.Notifier.HandleDomainEvent(event)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrEventEmissionFailed, err)
	}

	return nil
}

// GetEffectiveSystemPrompt returns the combined system prompt including
// all additions from middleware.
func (rc *RequestContext) GetEffectiveSystemPrompt() string {
	if len(rc.SystemPromptAdditions) == 0 {
		return rc.SystemPrompt
	}

	result := rc.SystemPrompt
	var resultSb146 strings.Builder
	for _, addition := range rc.SystemPromptAdditions {
		if addition != "" {
			resultSb146.WriteString("\n\n" + addition)
		}
	}
	result += resultSb146.String()

	return result
}

// GetEffectiveTools returns all tools including those added by middleware.
func (rc *RequestContext) GetEffectiveTools() []agent.ExecutableTool {
	if len(rc.AdditionalTools) == 0 {
		return rc.Tools
	}

	allTools := make([]agent.ExecutableTool, 0, len(rc.Tools)+len(rc.AdditionalTools))
	allTools = append(allTools, rc.Tools...)
	allTools = append(allTools, rc.AdditionalTools...)

	return allTools
}

// ContextOption is a function that modifies a RequestContext.
// This enables the builder pattern for context modifications.
type ContextOption func(*RequestContext)

// WithMessages sets the messages in the context.
func WithMessages(messages []llm.Message) ContextOption {
	return func(rc *RequestContext) {
		rc.Messages = messages
	}
}

// WithSystemPrompt sets the system prompt in the context.
func WithSystemPrompt(prompt string) ContextOption {
	return func(rc *RequestContext) {
		rc.SystemPrompt = prompt
	}
}

// WithTools sets the tools in the context.
func WithTools(tools []agent.ExecutableTool) ContextOption {
	return func(rc *RequestContext) {
		rc.Tools = tools
	}
}

// WithResult sets the result in the context.
func WithResult(result any) ContextOption {
	return func(rc *RequestContext) {
		rc.Result = result
	}
}

// Override creates a new RequestContext with the given options applied.
// This follows an immutable pattern, leaving the original context unchanged.
func (rc *RequestContext) Override(opts ...ContextOption) *RequestContext {
	// Create a shallow copy
	newCtx := &RequestContext{
		ThreadID:              rc.ThreadID,
		Messages:              rc.Messages,
		SystemPrompt:          rc.SystemPrompt,
		Tools:                 rc.Tools,
		Config:                rc.Config,
		State:                 rc.State, // Share the same state map
		Result:                rc.Result,
		Terminate:             rc.Terminate,
		AdditionalTools:       rc.AdditionalTools,
		SystemPromptAdditions: rc.SystemPromptAdditions,
	}

	// Apply options
	for _, opt := range opts {
		opt(newCtx)
	}

	return newCtx
}
