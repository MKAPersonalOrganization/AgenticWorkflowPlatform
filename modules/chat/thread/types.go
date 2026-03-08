package thread

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	es "school-with-ai/foundation/eventsourcing"
)

// Common errors.
var (
	// ErrThreadNotFound is returned when an aggregate has no events in the store.
	// It is an alias for eventsourcing.ErrNotFound so that errors.Is works for
	// both thread.ErrThreadNotFound and eventsourcing.ErrNotFound.
	ErrThreadNotFound      = es.ErrNotFound
	ErrConcurrencyConflict = es.ErrConcurrencyConflict
)

// ──────────────────────────────────────────────────────────────────────────────
// Eventsourcing re-exports — convenience aliases so that callers dealing with
// the thread aggregate only need to import this one package.
// ──────────────────────────────────────────────────────────────────────────────

// Event is the base interface for all domain events.
type Event = es.Event

// EventEnvelope wraps an event with metadata about its position in the stream.
type EventEnvelope = es.EventEnvelope

// Aggregate is the base interface for all event-sourced aggregates.
type Aggregate = es.Aggregate

// BaseAggregate provides common functionality for event-sourced aggregates.
type BaseAggregate = es.BaseAggregate

// StreamName generates a consistent stream name for an aggregate.
var StreamName = es.StreamName

// EventStore interface defines the contract for persisting events.
type EventStore = es.EventStore

// Repository provides a generic interface for working with any aggregate.
type Repository[T interface {
	Aggregate
	LoadFromHistory(events []Event, apply func(Event)) error
}] = es.Repository[T]

// NewRepository creates a new generic repository for any aggregate type.
func NewRepository[T interface {
	Aggregate
	LoadFromHistory(events []Event, apply func(Event)) error
}](
	eventStore EventStore,
	newAggregate func(id string) T,
	getStreamName func(T) string,
) *Repository[T] {
	return es.NewRepository(eventStore, newAggregate, getStreamName)
}

// ToolCall represents a tool/function call made by the assistant.
type ToolCall struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // "function"
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Message represents a single message in a conversation.
type Message struct {
	ID         string         `json:"id"`
	Role       string         `json:"role"` // "user" | "assistant" | "tool"
	Content    string         `json:"content"`
	Timestamp  time.Time      `json:"timestamp"`
	ToolCalls  []ToolCall     `json:"toolCalls,omitempty"`  // For assistant messages with tool calls
	ToolCallID string         `json:"toolCallId,omitempty"` // For tool result messages
	ToolName   string         `json:"toolName,omitempty"`   // Name of the tool for result messages
	IsError    bool           `json:"isError,omitempty"`    // For tool result messages
	Error      string         `json:"error,omitempty"`      // For tool result messages
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

// GenerateMessageID generates a unique message ID.
func GenerateMessageID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// This should never happen, but if it does, panic is appropriate
		// since we can't generate a secure random ID
		panic(fmt.Sprintf("failed to generate random ID: %s", err))
	}

	return hex.EncodeToString(b)
}

// ToolCallState represents the current state of a tool call in the thread.
type ToolCallState struct {
	ToolCallID  string         `json:"toolCallId"`
	ToolName    string         `json:"toolName,omitempty"`
	AgentName   string         `json:"agentName,omitempty"`
	Status      string         `json:"status"` // "AwaitingInput", "AwaitingAuth", "Completed", etc.
	Elicitation map[string]any `json:"elicitation,omitempty"`
	AuthSpec    map[string]any `json:"authSpec,omitempty"`
}

// ElicitationSpec defines the structure for user input requests.
type ElicitationSpec struct {
	Question string         `json:"question"`
	Schema   map[string]any `json:"schema,omitempty"`
}

// AuthorizationSpec defines a request for user authorization.
type AuthorizationSpec struct {
	Resource string   `json:"resource"` // What's being accessed
	Action   string   `json:"action"`   // What action is requested
	Reason   string   `json:"reason"`   // Why this is needed
	Scopes   []string `json:"scopes"`   // Required permission scopes
}
