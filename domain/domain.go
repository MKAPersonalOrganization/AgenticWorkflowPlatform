package domain

import "time"

// Message represents a single message in a conversation
type Message struct {
	ID         string                 `json:"id"`
	Role       string                 `json:"role"` // "user" | "assistant" | "tool"
	Content    string                 `json:"content"`
	Timestamp  time.Time              `json:"timestamp"`
	ToolCalls  []ToolCall             `json:"toolCalls,omitempty"`  // For assistant messages with tool calls
	ToolCallID string                 `json:"toolCallId,omitempty"` // For tool result messages
	AgentName  string                 `json:"agentName,omitempty"`  // Name of the agent that created this message
	Metadata   map[string]interface{} `json:"metadata,omitempty"`   // Additional metadata for custom message types
}

// ToolCall represents a tool/function call made by the assistant
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function details of a tool call
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Thread represents a conversation thread with message history
type Thread struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Multi-agent instructor fields
	CurrentInstructor string              `json:"currentInstructor,omitempty"`
	InstructorHistory []InstructorSession `json:"instructorHistory,omitempty"`
	WhiteboardContent string              `json:"whiteboardContent,omitempty"`
}

// InstructorSession tracks when an instructor was active
type InstructorSession struct {
	InstructorName string     `json:"instructorName"`
	Specialization string     `json:"specialization,omitempty"`
	StartedAt      time.Time  `json:"startedAt"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	HandoffReason  string     `json:"handoffReason,omitempty"`
}
