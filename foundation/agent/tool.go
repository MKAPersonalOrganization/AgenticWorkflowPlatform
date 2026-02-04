package agent

import (
	"context"
	"errors"
)

// ErrConfirmationRequired is returned when a tool requires user confirmation.
var ErrConfirmationRequired = errors.New("tool requires user confirmation")

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

// ExtendedTool provides additional capabilities beyond the base Tool interface.
// Tools can optionally implement this interface for enhanced functionality.
type ExtendedTool interface {
	Tool

	// OutputSchema returns the JSON schema for tool output (optional).
	// Returns nil if output schema is not defined.
	OutputSchema() map[string]any

	// IsLongRunning indicates if the tool is a long-running operation.
	// Long-running tools return a resource ID first and complete later.
	IsLongRunning() bool
}

// ConfirmableTool represents a tool that may require user confirmation.
// This supports Human-in-the-Loop (HITL) patterns from ADK-Go.
type ConfirmableTool interface {
	Tool

	// RequiresConfirmation returns true if this tool requires user confirmation
	// before execution for the given arguments.
	RequiresConfirmation(ctx context.Context, args map[string]any) bool
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

	// Artifacts provides access to artifact storage (optional).
	Artifacts Artifacts

	// Memory provides access to long-term memory (optional).
	Memory Memory

	// Actions allows the tool to request state changes.
	// Keys are action names, values are action data.
	Actions map[string]any

	// Confirmation contains user confirmation if this is a HITL tool.
	// nil if no confirmation was requested or provided.
	Confirmation *ToolConfirmation

	// Metadata for framework-specific data.
	Metadata map[string]any
}

// ToolConfirmation represents user confirmation for a tool execution.
type ToolConfirmation struct {
	// Approved indicates whether the user approved the action.
	Approved bool

	// Hint is a human-readable explanation of what needs confirmation.
	Hint string

	// Payload contains the action details that were confirmed/rejected.
	Payload any
}

// RequestConfirmation creates a ToolResult that requests user confirmation.
// The tool execution will pause until the user responds.
func (tc *ToolContext) RequestConfirmation(hint string, payload any) *ToolResult {
	return &ToolResult{
		ToolCallID:           tc.ToolCallID,
		RequiresConfirmation: true,
		ConfirmationHint:     hint,
		ConfirmationPayload:  payload,
	}
}

// Toolset groups related tools together.
// This is compatible with ADK-Go's tool.Toolset interface.
type Toolset interface {
	// Name returns the toolset identifier.
	Name() string

	// Tools returns the tools in this set.
	// The context can be used to dynamically filter tools based on invocation state.
	Tools(ctx ReadonlyContext) ([]Tool, error)
}

// ToolResolver dynamically resolves tools from external sources.
// This supports MCP (Model Context Protocol) and plugin-based tools.
type ToolResolver interface {
	// Resolve returns tools available from this resolver.
	Resolve(ctx context.Context) ([]Tool, error)
}

// ToolPredicate filters tools based on context.
// Returns true if the tool should be available.
type ToolPredicate func(ctx ReadonlyContext, t Tool) bool

// AllowedTools creates a ToolPredicate that allows only the named tools.
func AllowedTools(names ...string) ToolPredicate {
	allowed := make(map[string]bool, len(names))
	for _, name := range names {
		allowed[name] = true
	}
	return func(_ ReadonlyContext, t Tool) bool {
		return allowed[t.Name()]
	}
}
