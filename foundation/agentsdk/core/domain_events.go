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

// Now provides a mockable time function for testing.
var Now = time.Now
