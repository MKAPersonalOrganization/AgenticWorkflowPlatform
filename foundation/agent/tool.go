package agent

import "context"

// Tool represents a function that an agent can invoke.
// This interface is designed to be compatible with:
// - foundation/agentsdk/core.Tool
// - ADK-Go tool.Tool
// - Blades tools.Tool
type Tool interface {
	// Name returns the tool's function name.
	Name() string

	// Description returns a human-readable description for the LLM.
	Description() string

	// InputSchema returns the JSON schema for tool parameters.
	// Must be compatible with JSON Schema draft-07 or OpenAPI 3.0.
	InputSchema() map[string]any

	// Execute runs the tool with the given context and arguments.
	// Arguments are passed as a JSON-encoded string for flexibility.
	Execute(context.Context, *ToolContext) (*ToolResult, error)
}

// ToolContext provides execution context for tools.
type ToolContext struct {
	// Context is the Go context for cancellation and deadlines.
	context.Context

	// SessionID identifies the current conversation.
	SessionID string

	// UserID identifies the user.
	UserID string

	// ToolCallID uniquely identifies this tool invocation.
	ToolCallID string

	// Arguments is the JSON-encoded tool input.
	Arguments string

	// Session provides access to conversation state.
	Session Session

	// Metadata for framework-specific data.
	Metadata map[string]any
}
