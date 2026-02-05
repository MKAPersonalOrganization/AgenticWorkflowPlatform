package agent

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrConfirmationRequired is returned when a tool requires user confirmation.
var ErrConfirmationRequired = errors.New("tool requires user confirmation")

// Schema describes the structure of data for tool parameters or structured output.
type Schema struct {
	Type        SchemaType         `json:"type"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`  // For arrays
	Enum        []string           `json:"enum,omitempty"`   // For enums
	Format      string             `json:"format,omitempty"` // date-time, email, etc.
	Default     any                `json:"default,omitempty"`
	Nullable    bool               `json:"nullable,omitempty"`
}

// SchemaType represents JSON Schema types.
type SchemaType string

const (
	TypeString  SchemaType = "string"
	TypeNumber  SchemaType = "number"
	TypeInteger SchemaType = "integer"
	TypeBoolean SchemaType = "boolean"
	TypeArray   SchemaType = "array"
	TypeObject  SchemaType = "object"
)

// Metadata holds arbitrary key-value pairs.
type Metadata = map[string]any

// Result represents the outcome of a tool execution per universal agent interface spec.
type Result struct {
	Content  []any    // Output content (can be string, Content, or any serializable type)
	Error    error    // Execution error, if any
	Metadata Metadata // Additional result metadata
}

// Tool represents metadata for a capability that an agent can invoke.
// This is the minimal interface - use ExecutableTool for tools that can be executed.
type Tool interface {
	// Name returns the tool's unique identifier.
	Name() string

	// Description returns a human-readable description for the LLM.
	Description() string

	// Schema returns the JSON Schema for the tool's parameters.
	Schema() *Schema
}

// Executor can execute tool invocations per universal agent interface spec.
type Executor interface {
	// Execute runs the tool with the given context and arguments.
	Execute(ctx context.Context, args Arguments) (Result, error)
}

// ExecutableTool combines Tool metadata with execution capability.
type ExecutableTool interface {
	Tool
	Executor
}

// Arguments holds the decoded arguments for a tool invocation.
type Arguments struct {
	Raw    json.RawMessage // Original JSON
	CallID string          // Unique ID for this invocation

	// Legacy fields for backward compatibility with ToolContext
	SessionID string
	UserID    string
	Session   Session
	Artifacts Artifacts
	Memory    Memory
	Metadata  map[string]any
}

// Decode unmarshals arguments into the target type.
func (a *Arguments) Decode(target any) error {
	return json.Unmarshal(a.Raw, target)
}

// NewResult creates a Result with content.
func NewResult(content ...any) Result {
	return Result{Content: content}
}

// NewResultWithMetadata creates a Result with content and metadata.
func NewResultWithMetadata(metadata Metadata, content ...any) Result {
	return Result{Content: content, Metadata: metadata}
}

// NewResultError creates a Result with an error.
func NewResultError(err error) Result {
	return Result{Error: err}
}

// TextResult creates a successful Result with text content.
func TextResult(text string) Result {
	return Result{Content: []any{text}}
}

// JSONResult creates a successful Result with JSON content.
func JSONResult(data any) Result {
	jsonBytes, _ := json.Marshal(data)
	return Result{Content: []any{string(jsonBytes)}}
}

// ErrorResult creates a failed Result.
func ErrorResult(err error) Result {
	return Result{Error: err}
}

// LongRunning tools return immediately with a handle for later completion.
type LongRunning interface {
	Tool
	// IsLongRunning returns true if this tool runs asynchronously.
	IsLongRunning() bool
}

// Confirmable tools require user confirmation before execution.
type Confirmable interface {
	Tool
	// RequiresConfirmation returns true if user must confirm.
	RequiresConfirmation(ctx context.Context, args Arguments) bool
	// ConfirmationMessage returns the message to show the user.
	ConfirmationMessage(args Arguments) string
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
