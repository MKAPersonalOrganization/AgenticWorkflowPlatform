// Package agent provides compatibility helpers for migrating from agentsdk.
package agent

import (
	"context"
)

// RequestMiddleware is a compatibility interface for agentsdk-style middleware.
// This allows services to define middleware without depending on agentsdk/core.
type RequestMiddleware interface {
	Process(ctx context.Context, reqCtx *RequestContext, next RequestNextHandler) error
}

// RequestNextHandler continues the middleware chain execution.
type RequestNextHandler func(ctx context.Context, reqCtx *RequestContext) error

// RequestContext carries all state through a request middleware chain.
// This is compatible with agentsdk/core.RequestContext for migration.
type RequestContext struct {
	// ThreadID is the identifier for the conversation thread.
	ThreadID string

	// Messages are the conversation messages.
	Messages []*Message

	// SystemPrompt is the system-level instruction.
	SystemPrompt string

	// Tools are the available tools for this request.
	Tools []Tool

	// Config is the agent configuration.
	Config *Config

	// State is a key-value store for sharing data between middleware.
	State map[string]any

	// Result contains the execution result after the handler runs.
	Result any

	// Terminate indicates whether to stop execution.
	Terminate bool

	// AdditionalTools are tools added by middleware.
	AdditionalTools []Tool

	// SystemPromptAdditions are system prompt fragments added by middleware.
	SystemPromptAdditions []string
}

// NewRequestContext creates a new RequestContext.
func NewRequestContext(threadID string, messages []*Message, systemPrompt string, tools []Tool, config *Config) *RequestContext {
	return &RequestContext{
		ThreadID:              threadID,
		Messages:              messages,
		SystemPrompt:          systemPrompt,
		Tools:                 tools,
		Config:                config,
		State:                 make(map[string]any),
		AdditionalTools:       []Tool{},
		SystemPromptAdditions: []string{},
		Terminate:             false,
	}
}

// AddSystemPrompt appends a system prompt fragment.
func (rc *RequestContext) AddSystemPrompt(prompt string) {
	rc.SystemPromptAdditions = append(rc.SystemPromptAdditions, prompt)
}

// AddTool adds a tool to the request.
func (rc *RequestContext) AddTool(tool Tool) {
	rc.AdditionalTools = append(rc.AdditionalTools, tool)
}

// GetState retrieves a value from the state map.
func (rc *RequestContext) GetState(key string) (any, bool) {
	val, ok := rc.State[key]
	return val, ok
}

// SetState sets a value in the state map.
func (rc *RequestContext) SetState(key string, value any) {
	rc.State[key] = value
}

// RequestMiddlewareFunc is a function adapter for RequestMiddleware.
type RequestMiddlewareFunc func(ctx context.Context, reqCtx *RequestContext, next RequestNextHandler) error

// Process implements RequestMiddleware for RequestMiddlewareFunc.
func (f RequestMiddlewareFunc) Process(ctx context.Context, reqCtx *RequestContext, next RequestNextHandler) error {
	return f(ctx, reqCtx, next)
}
