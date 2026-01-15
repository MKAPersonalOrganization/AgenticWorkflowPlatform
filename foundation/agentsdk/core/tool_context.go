package core

import (
	"context"
)

// ToolContext provides tools with access to execution context beyond standard context.Context.
// It extends context.Context with agent-specific information that tools need.
// This follows the ADK-Go pattern where tools receive a rich context object.
type ToolContext struct {
	context.Context

	// ThreadID is the identifier for the current conversation thread
	ThreadID string

	// MessageID is the identifier for the current message being processed
	MessageID string

	// ToolCallID is the identifier for this specific tool call
	ToolCallID string

	// RequestCtx provides access to the request context, including state and config
	RequestCtx *RequestContext
}

// NewToolContext creates a new ToolContext with the given data.
func NewToolContext(
	ctx context.Context,
	threadID string,
	messageID string,
	toolCallID string,
	requestCtx *RequestContext,
) ToolContext {
	return ToolContext{
		Context:    ctx,
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		RequestCtx: requestCtx,
	}
}

// State returns the request context state (read-only access).
// Returns nil if request context is not available.
func (tc ToolContext) State() map[string]any {
	if tc.RequestCtx != nil {
		return tc.RequestCtx.State
	}
	return nil
}

// AgentName returns the current agent name, or empty string if not available.
func (tc ToolContext) AgentName() string {
	if tc.RequestCtx != nil && tc.RequestCtx.Config != nil {
		return tc.RequestCtx.Config.Name
	}
	return ""
}
