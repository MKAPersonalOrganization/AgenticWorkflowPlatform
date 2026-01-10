package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateMessageID generates a unique message ID.
func GenerateMessageID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// This should never happen, but if it does, panic is appropriate
		// since we can't generate a secure random ID
		panic(fmt.Sprintf("failed to generate random ID: %v", err))
	}

	return hex.EncodeToString(b)
}

// Message represents a single message in a conversation.
type Message struct {
	ID         string         `json:"id"`
	Role       string         `json:"role"` // "user" | "assistant" | "tool"
	Content    string         `json:"content"`
	Timestamp  time.Time      `json:"timestamp"`
	ToolCalls  []ToolCall     `json:"toolCalls,omitempty"`  // For assistant messages with tool calls
	ToolCallID string         `json:"toolCallId,omitempty"` // For tool result messages
	AgentName  string         `json:"agentName,omitempty"`  // Name of the agent that created this message
	Metadata   map[string]any `json:"metadata,omitempty"`   // Additional metadata for custom message types
}

// Thread represents a conversation thread with message history.
type Thread struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToolCall represents a tool/function call made by the assistant.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction represents the function details of a tool call.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Event is the base interface for all domain events.
type Event interface {
	EventType() string
	EventTimestamp() time.Time
	AggregateID() string // ThreadID
}

// BaseEvent provides common fields for all events.
type BaseEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	ThreadID  string    `json:"threadId"`
}

func (e BaseEvent) EventType() string         { return e.Type }
func (e BaseEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e BaseEvent) AggregateID() string       { return e.ThreadID }

// ThreadCreatedEvent is emitted when a new thread is created.
type ThreadCreatedEvent struct {
	BaseEvent

	Title string `json:"title"`
}

// UserMessageAddedEvent is emitted when a user sends a message.
type UserMessageAddedEvent struct {
	BaseEvent

	MessageID string `json:"messageId"`
	Content   string `json:"content"`
}

// AssistantMessageStartedEvent is emitted when the assistant begins responding.
type AssistantMessageStartedEvent struct {
	BaseEvent

	MessageID string `json:"messageId"`
}

// AssistantContentStreamedEvent is emitted for each content delta during streaming.
type AssistantContentStreamedEvent struct {
	BaseEvent

	MessageID string `json:"messageId"`
	Delta     string `json:"delta"`
}

// ToolCallStartedEvent is emitted when a tool call begins.
type ToolCallStartedEvent struct {
	BaseEvent

	ToolCallID      string `json:"toolCallId"`
	ToolCallName    string `json:"toolCallName"`
	ParentMessageID string `json:"parentMessageId"`
	AgentName       string `json:"agentName"`
}

// ToolCallArgumentsStreamedEvent is emitted for each argument delta during streaming.
type ToolCallArgumentsStreamedEvent struct {
	BaseEvent

	ToolCallID string `json:"toolCallId"`
	Delta      string `json:"delta"`
}

// ToolCallCompletedEvent is emitted when a tool call's arguments are complete.
type ToolCallCompletedEvent struct {
	BaseEvent

	ToolCallID string `json:"toolCallId"`
}

// AssistantMessageCompletedEvent is emitted when the assistant completes a message.
type AssistantMessageCompletedEvent struct {
	BaseEvent

	MessageID string     `json:"messageId"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	AgentName string     `json:"agentName"`
}

// ToolCallExecutedEvent is emitted when a tool call is executed.
type ToolCallExecutedEvent struct {
	BaseEvent

	MessageID       string `json:"messageId"` // ID of the tool result message
	ToolCallID      string `json:"toolCallId"`
	ToolName        string `json:"toolName"`
	ToolArguments   string `json:"toolArguments"`
	ToolResult      string `json:"toolResult"`
	ParentMessageID string `json:"parentMessageId"` // ID of the assistant message that initiated the tool call
	AgentName       string `json:"agentName"`
}

// ThreadTitleUpdatedEvent is emitted when a thread's title is updated.
type ThreadTitleUpdatedEvent struct {
	BaseEvent

	NewTitle string `json:"newTitle"`
}

// ThreadDeletedEvent is emitted when a thread is deleted.
type ThreadDeletedEvent struct {
	BaseEvent
}

// Now provides a mockable time function for testing.
var Now = time.Now
