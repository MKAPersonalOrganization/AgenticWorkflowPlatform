package domain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// Common errors
var (
	ErrThreadNotFound = errors.New("thread not found")
)

// GenerateMessageID generates a unique message ID
func GenerateMessageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // crypto/rand.Read always returns len(b), nil on success
	return hex.EncodeToString(b)
}

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
}
