package agent

import (
	"time"

	"school-with-ai/domain"
)

// Event is the base interface for all domain events
type Event interface {
	EventType() string
	EventTimestamp() time.Time
	AggregateID() string // ThreadID
}

// BaseEvent provides common fields for all events
type BaseEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	ThreadID  string    `json:"threadId"`
}

func (e BaseEvent) EventType() string         { return e.Type }
func (e BaseEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e BaseEvent) AggregateID() string       { return e.ThreadID }

// ThreadCreatedEvent is emitted when a new thread is created
type ThreadCreatedEvent struct {
	BaseEvent
	Title string `json:"title"`
}

// UserMessageAddedEvent is emitted when a user sends a message
type UserMessageAddedEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
	Content   string `json:"content"`
}

// AssistantMessageStartedEvent is emitted when the assistant begins responding
type AssistantMessageStartedEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
}

// AssistantContentStreamedEvent is emitted for each content delta during streaming
type AssistantContentStreamedEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
	Delta     string `json:"delta"`
}

// ToolCallStartedEvent is emitted when a tool call begins
type ToolCallStartedEvent struct {
	BaseEvent
	ToolCallID      string `json:"toolCallId"`
	ToolCallName    string `json:"toolCallName"`
	ParentMessageID string `json:"parentMessageId"`
}

// ToolCallArgumentsStreamedEvent is emitted for each argument delta during streaming
type ToolCallArgumentsStreamedEvent struct {
	BaseEvent
	ToolCallID string `json:"toolCallId"`
	Delta      string `json:"delta"`
}

// ToolCallCompletedEvent is emitted when a tool call's arguments are complete
type ToolCallCompletedEvent struct {
	BaseEvent
	ToolCallID string `json:"toolCallId"`
}

// AssistantMessageCompletedEvent is emitted when the assistant completes a message
type AssistantMessageCompletedEvent struct {
	BaseEvent
	MessageID string            `json:"messageId"`
	Content   string            `json:"content"`
	ToolCalls []domain.ToolCall `json:"toolCalls,omitempty"`
}

// ToolCallExecutedEvent is emitted when a tool call is executed
type ToolCallExecutedEvent struct {
	BaseEvent
	MessageID       string `json:"messageId"` // ID of the tool result message
	ToolCallID      string `json:"toolCallId"`
	ToolName        string `json:"toolName"`
	ToolArguments   string `json:"toolArguments"`
	ToolResult      string `json:"toolResult"`
	ParentMessageID string `json:"parentMessageId"` // ID of the assistant message that initiated the tool call
}

// ThreadTitleUpdatedEvent is emitted when a thread's title is updated
type ThreadTitleUpdatedEvent struct {
	BaseEvent
	NewTitle string `json:"newTitle"`
}

// ThreadDeletedEvent is emitted when a thread is deleted
type ThreadDeletedEvent struct {
	BaseEvent
}
