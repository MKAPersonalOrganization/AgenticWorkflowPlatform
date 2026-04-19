package agent

import (
	"context"
	"encoding/json"
)

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

	// Actions optionally carries runtime directives for the framework to apply
	// after this tool call completes (e.g. TransferToAgent, Escalate, StateDelta).
	// Mirrors ADK's tool.Context.Actions() — tools signal routing by bundling
	// directives with their result rather than mutating external state.
	// nil means no directives.
	Actions *EventActions
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

// StreamCallback is called by streaming tools to emit content deltas in real-time.
// The toolCallID identifies which tool call the delta belongs to.
// The delta is a chunk of the tool's output (e.g., a token from an LLM response).
// The field identifies what part of the result is being streamed (e.g., "answer", "thoughts").
// Returns false if the stream consumer has disconnected.
type StreamCallback func(delta string, field string) bool

// StreamingExecutor can execute tool invocations with real-time streaming output.
type StreamingExecutor interface {
	// ExecuteStreaming runs the tool with the given context, arguments, and a callback
	// for emitting content deltas in real-time. The final Result is still returned
	// at the end for persistence, but intermediate tokens are streamed via the callback.
	ExecuteStreaming(ctx context.Context, args Arguments, stream StreamCallback) (Result, error)
}

// StreamingExecutableTool combines Tool metadata with streaming execution capability.
// Tools that implement this interface will have their output streamed token-by-token
// to the frontend, instead of waiting for the complete result.
type StreamingExecutableTool interface {
	Tool
	StreamingExecutor
}

// LongRunningTool is implemented by tools that initiate a background operation
// and return immediately with a status identifier, completing asynchronously.
// Aligned with ADK-Go's tool.Tool.IsLongRunning() semantics.
//
// When a tool implements this interface and returns true, the adapter will mark
// the corresponding ADK-Go function tool as long-running, causing the LLM to
// avoid re-calling the tool if it has already returned a pending status.
type LongRunningTool interface {
	// IsLongRunning returns true if this tool starts a long-running operation.
	IsLongRunning() bool
}

// IsLongRunningTool returns true if the tool implements LongRunningTool and
// reports itself as long running. Returns false for all other tools.
func IsLongRunningTool(t Tool) bool {
	if lrt, ok := t.(LongRunningTool); ok {
		return lrt.IsLongRunning()
	}
	return false
}

// Toolset provides a dynamic, named collection of tools resolved per invocation.
// This mirrors ADK-Go's tool.Toolset interface, enabling context-sensitive tool
// sets such as MCP servers, A2A agents, or user-enabled plugin groups.
//
// Unlike a static []Tool slice in Config, a Toolset can filter tools based on
// runtime state (permissions, feature flags, user context, etc.).
//
// Example:
//
//	type CalculatorToolset struct{}
//
//	func (s *CalculatorToolset) Name() string { return "calculator" }
//
//	func (s *CalculatorToolset) Tools(ctx context.Context) ([]ExecutableTool, error) {
//	    return []ExecutableTool{NewAddTool(), NewSubtractTool()}, nil
//	}
type Toolset interface {
	// Name returns the toolset's unique identifier.
	Name() string

	// Tools returns the available tools for the current invocation.
	// Implementations may filter dynamically based on the context.
	Tools(ctx context.Context) ([]ExecutableTool, error)
}

// Arguments holds the decoded arguments for a tool invocation.
type Arguments struct {
	Raw    json.RawMessage // Original JSON
	CallID string          // Unique ID for this invocation

	// Context fields populated by the executor.
	SessionID string
	UserID    string
	Artifacts Artifacts
	Metadata  map[string]any
}

// Decode unmarshals arguments into the target type.
func (a *Arguments) Decode(target any) error {
	return json.Unmarshal(a.Raw, target)
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
