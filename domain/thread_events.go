package domain

import (
	"school-with-ai/foundation/agentsdk/core"
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
