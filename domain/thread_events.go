package domain

import (
	"school-with-ai/sdk/agentsdk/core"
	"time"
)

// Thread domain events

// ThreadCreated is emitted when a new thread is created.
type ThreadCreated struct {
	ThreadID  string    `json:"threadId"`
	UserID    string    `json:"userId"`
	Title     string    `json:"title,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (e ThreadCreated) EventType() string         { return "ThreadCreated" }
func (e ThreadCreated) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadCreated) AggregateID() string       { return e.ThreadID }

// MessageAdded is emitted when a message is added to the thread.
type MessageAdded struct {
	ThreadID   string          `json:"threadId"`
	MessageID  string          `json:"messageId"`
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCalls  []core.ToolCall `json:"toolCalls,omitempty"`
	ToolCallID string          `json:"toolCallId,omitempty"`
	AgentName  string          `json:"agentName,omitempty"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

func (e MessageAdded) EventType() string         { return "MessageAdded" }
func (e MessageAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e MessageAdded) AggregateID() string       { return e.ThreadID }

// ThreadTitleChanged is emitted when a thread's title is updated.
type ThreadTitleChanged struct {
	ThreadID  string    `json:"threadId"`
	NewTitle  string    `json:"newTitle"`
	Timestamp time.Time `json:"timestamp"`
}

func (e ThreadTitleChanged) EventType() string         { return "ThreadTitleChanged" }
func (e ThreadTitleChanged) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadTitleChanged) AggregateID() string       { return e.ThreadID }

// ThreadDeleted is emitted when a thread is deleted.
type ThreadDeleted struct {
	ThreadID  string    `json:"threadId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e ThreadDeleted) EventType() string         { return "ThreadDeleted" }
func (e ThreadDeleted) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadDeleted) AggregateID() string       { return e.ThreadID }

// MessagesCleared is emitted when all messages in a thread are cleared.
type MessagesCleared struct {
	ThreadID  string    `json:"threadId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e MessagesCleared) EventType() string         { return "MessagesCleared" }
func (e MessagesCleared) EventTimestamp() time.Time { return e.Timestamp }
func (e MessagesCleared) AggregateID() string       { return e.ThreadID }

// ToolCallExecuted is emitted when a tool call is executed.
// This event is persisted to enable projection rebuilding (e.g., whiteboard state).
type ToolCallExecuted struct {
	ThreadID        string    `json:"threadId"`
	MessageID       string    `json:"messageId"` // ID of the tool result message
	ToolCallID      string    `json:"toolCallId"`
	ToolName        string    `json:"toolName"`
	ToolArguments   string    `json:"toolArguments"`
	ToolResult      string    `json:"toolResult"`
	ParentMessageID string    `json:"parentMessageId"` // ID of the assistant message that initiated the tool call
	AgentName       string    `json:"agentName"`
	Timestamp       time.Time `json:"timestamp"`
}

func (e ToolCallExecuted) EventType() string         { return "ToolCallExecuted" }
func (e ToolCallExecuted) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolCallExecuted) AggregateID() string       { return e.ThreadID }
