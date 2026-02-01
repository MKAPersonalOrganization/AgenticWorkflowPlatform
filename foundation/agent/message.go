package agent

import "time"

// Message represents a single message in a conversation.
// Compatible with both agent frameworks and LLM APIs.
type Message struct {
	// ID is a unique identifier for this message.
	ID string

	// Role indicates who sent the message (user, assistant, system, tool).
	Role MessageRole

	// Content is the message text or structured content.
	Content []Content

	// Timestamp when the message was created.
	Timestamp time.Time

	// ToolCallID links tool results to their originating call.
	ToolCallID string

	// Metadata for extensibility (e.g., token usage, model info).
	Metadata map[string]any
}

// MessageRole identifies the sender of a message.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// Content represents a piece of message content (text, tool call, etc.).
type Content struct {
	// Type indicates the content type (text, tool_call, tool_result, etc.).
	Type ContentType

	// Text is the textual content (for Type == ContentTypeText).
	Text string

	// ToolCall is the tool invocation (for Type == ContentTypeToolCall).
	ToolCall *ToolCall

	// ToolResult is the tool output (for Type == ContentTypeToolResult).
	ToolResult *ToolResult
}

// ContentType identifies the type of content.
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeToolCall   ContentType = "tool_call"
	ContentTypeToolResult ContentType = "tool_result"
)

// ToolCall represents a request to execute a tool.
type ToolCall struct {
	// ID uniquely identifies this tool call.
	ID string `json:"id"`

	// Name is the tool function name.
	Name string `json:"name"`

	// Arguments is the JSON-encoded tool input.
	Arguments string `json:"arguments"`
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
	// ToolCallID links this result to the originating call.
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Output is the JSON-encoded tool output.
	Output string `json:"output,omitempty"`

	// Error contains error information if the tool failed.
	Error string `json:"error,omitempty"`

	// IsError indicates whether the execution failed.
	IsError bool `json:"is_error,omitempty"`
}
