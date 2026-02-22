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

// IsStreamingTool checks if an ExecutableTool also implements StreamingExecutableTool.
func IsStreamingTool(tool ExecutableTool) (StreamingExecutableTool, bool) {
	st, ok := tool.(StreamingExecutableTool)
	return st, ok
}

// Arguments holds the decoded arguments for a tool invocation.
type Arguments struct {
	Raw    json.RawMessage // Original JSON
	CallID string          // Unique ID for this invocation

	// Context fields populated by the executor.
	SessionID string
	UserID    string
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
