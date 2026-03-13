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

	// Status indicates the message generation state (for streaming).
	Status MessageStatus

	// FinishReason indicates why generation stopped.
	FinishReason FinishReason

	// ToolCallID links tool results to their originating call.
	ToolCallID string

	// Author identifies who created this message (agent name or "user").
	Author string

	// InvocationID links the message to its originating invocation.
	InvocationID string

	// Usage contains token consumption metrics.
	Usage *Usage

	// Metadata for extensibility (e.g., model info).
	Metadata map[string]any
}

// IsComplete returns true if the message is fully generated.
func (m *Message) IsComplete() bool {
	return m.Status == StatusCompleted
}

// IsPartial returns true if the message is still being generated.
func (m *Message) IsPartial() bool {
	return m.Status == StatusInProgress
}

// Text returns the concatenated text content of the message.
func (m *Message) Text() string {
	var text string
	for _, c := range m.Content {
		if c.Type == ContentTypeText {
			text += c.Text
		}
	}
	return text
}

// Clone creates a shallow copy of the message.
func (m *Message) Clone() *Message {
	if m == nil {
		return nil
	}
	copy := *m
	return &copy
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

	// ImageURL is an image URL or base64 data URI (for Type == ContentTypeImageURL).
	ImageURL *ImageURL

	// AudioInput is base64-encoded audio data (for Type == ContentTypeInputAudio).
	AudioInput *AudioInput

	// FileInput is base64-encoded file data (for Type == ContentTypeFile).
	FileInput *FileInput
}

// ImageURL represents an image reference in a message.
type ImageURL struct {
	URL    string `json:"url"`              // HTTP URL or data:image/...;base64,... data URI
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// AudioInput represents base64-encoded audio data for multimodal messages.
type AudioInput struct {
	Data   string `json:"data"`   // Raw base64-encoded audio data (no data URL prefix)
	Format string `json:"format"` // "wav" or "mp3"
}

// FileInput represents base64-encoded file data for multimodal messages.
type FileInput struct {
	Data     string `json:"data"`     // Raw base64-encoded file data (no data URL prefix)
	FileName string `json:"fileName"` // Original filename
	MimeType string `json:"mimeType"` // e.g. "application/pdf", "text/plain"
}

// ContentType identifies the type of content.
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeToolCall   ContentType = "tool_call"
	ContentTypeToolResult ContentType = "tool_result"
	ContentTypeImageURL   ContentType = "image_url"
	ContentTypeInputAudio ContentType = "input_audio"
	ContentTypeFile       ContentType = "file"
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

	// ToolName is the name of the tool that was executed.
	ToolName string `json:"tool_name,omitempty"`

	// Output is the JSON-encoded tool output.
	Output string `json:"output,omitempty"`

	// Error contains error information if the tool failed.
	Error string `json:"error,omitempty"`

	// IsError indicates whether the execution failed.
	IsError bool `json:"is_error,omitempty"`
}

// NewUserMessage creates a new user message with text content.
func NewUserMessage(id string, text string) *Message {
	return &Message{
		ID:      id,
		Role:    RoleUser,
		Author:  "user",
		Status:  StatusCompleted,
		Content: []Content{{Type: ContentTypeText, Text: text}},
	}
}

// NewAssistantMessage creates a new assistant message.
func NewAssistantMessage(id string, status MessageStatus) *Message {
	return &Message{
		ID:       id,
		Role:     RoleAssistant,
		Status:   status,
		Content:  []Content{},
		Metadata: make(map[string]any),
	}
}

// NewSystemMessage creates a new system message.
func NewSystemMessage(text string) *Message {
	return &Message{
		ID:      GenerateID(),
		Role:    RoleSystem,
		Status:  StatusCompleted,
		Content: []Content{{Type: ContentTypeText, Text: text}},
	}
}

// NewToolResultMessage creates a new tool result message.
func NewToolResultMessage(id string, result *ToolResult) *Message {
	return &Message{
		ID:         id,
		Role:       RoleTool,
		Status:     StatusCompleted,
		ToolCallID: result.ToolCallID,
		Content:    []Content{{Type: ContentTypeToolResult, ToolResult: result}},
	}
}
