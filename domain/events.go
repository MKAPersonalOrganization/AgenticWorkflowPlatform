package domain

import (
	"time"
)

// Domain events - transport-agnostic events that represent business domain occurrences.
// These events are emitted by the orchestration service and can be converted to
// transport-specific events (AGUI, A2A, etc.) by the presentation layer.

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
	ThreadID   string         `json:"threadId"`
	MessageID  string         `json:"messageId"`
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []ToolCall     `json:"toolCalls,omitempty"`
	ToolCallID string         `json:"toolCallId,omitempty"`
	AgentName  string         `json:"agentName,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

func (e MessageAdded) EventType() string         { return "MessageAdded" }
func (e MessageAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e MessageAdded) AggregateID() string       { return e.ThreadID }

// ThreadTitleUpdatedEvent indicates that a thread's title has been changed.
type ThreadTitleUpdatedEvent struct {
	ThreadID  string    `json:"threadId"`
	Title     string    `json:"title"`
	Timestamp time.Time `json:"timestamp"`
}

func (e ThreadTitleUpdatedEvent) EventType() string         { return "ThreadTitleUpdated" }
func (e ThreadTitleUpdatedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadTitleUpdatedEvent) AggregateID() string       { return e.ThreadID }

// AssistantMessageStartedEvent is emitted when the assistant begins responding.
type AssistantMessageStartedEvent struct {
	ThreadID  string
	MessageID string
	Timestamp time.Time
}

func (e AssistantMessageStartedEvent) EventType() string         { return "AssistantMessageStarted" }
func (e AssistantMessageStartedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantMessageStartedEvent) AggregateID() string       { return e.ThreadID }

// AssistantContentStreamedEvent is emitted for each content delta during streaming.
type AssistantContentStreamedEvent struct {
	ThreadID  string
	MessageID string
	Delta     string
	Timestamp time.Time
}

func (e AssistantContentStreamedEvent) EventType() string         { return "AssistantContentStreamed" }
func (e AssistantContentStreamedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantContentStreamedEvent) AggregateID() string       { return e.ThreadID }

// AssistantMessageCompletedEvent is emitted when the assistant completes a message.
type AssistantMessageCompletedEvent struct {
	ThreadID  string
	MessageID string
	Content   string
	AgentName string
	ToolCalls []ToolCall
	Timestamp time.Time
}

func (e AssistantMessageCompletedEvent) EventType() string         { return "AssistantMessageCompleted" }
func (e AssistantMessageCompletedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantMessageCompletedEvent) AggregateID() string       { return e.ThreadID }

// ToolCallStartedEvent is emitted when a tool call begins.
type ToolCallStartedEvent struct {
	ThreadID        string
	ToolCallID      string
	ToolCallName    string
	ParentMessageID string
	AgentName       string
	Timestamp       time.Time
}

func (e ToolCallStartedEvent) EventType() string         { return "ToolCallStarted" }
func (e ToolCallStartedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolCallStartedEvent) AggregateID() string       { return e.ThreadID }

// ToolCallArgumentsStreamedEvent is emitted for each argument delta during streaming.
type ToolCallArgumentsStreamedEvent struct {
	ThreadID   string
	ToolCallID string
	Delta      string
	Timestamp  time.Time
}

func (e ToolCallArgumentsStreamedEvent) EventType() string         { return "ToolCallArgumentsStreamed" }
func (e ToolCallArgumentsStreamedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolCallArgumentsStreamedEvent) AggregateID() string       { return e.ThreadID }

// ToolCallCompletedEvent is emitted when a tool call's arguments are complete.
type ToolCallCompletedEvent struct {
	ThreadID   string
	ToolCallID string
	Timestamp  time.Time
}

func (e ToolCallCompletedEvent) EventType() string         { return "ToolCallCompleted" }
func (e ToolCallCompletedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolCallCompletedEvent) AggregateID() string       { return e.ThreadID }

// ToolCallExecutedEvent is emitted when a tool call is executed.
type ToolCallExecutedEvent struct {
	ThreadID        string
	MessageID       string
	ToolCallID      string
	ToolName        string
	ToolArguments   string
	ToolResult      string
	ParentMessageID string
	AgentName       string
	Timestamp       time.Time
}

func (e ToolCallExecutedEvent) EventType() string         { return "ToolCallExecuted" }
func (e ToolCallExecutedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolCallExecutedEvent) AggregateID() string       { return e.ThreadID }

// RunStartedEvent indicates that message processing has begun.
type RunStartedEvent struct {
	ThreadID  string
	RunID     string
	Timestamp time.Time
}

func (e RunStartedEvent) EventType() string         { return "RunStarted" }
func (e RunStartedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e RunStartedEvent) AggregateID() string       { return e.ThreadID }

// RunFinishedEvent indicates that message processing has completed.
type RunFinishedEvent struct {
	ThreadID  string
	RunID     string
	Timestamp time.Time
}

func (e RunFinishedEvent) EventType() string         { return "RunFinished" }
func (e RunFinishedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e RunFinishedEvent) AggregateID() string       { return e.ThreadID }

// RunErrorEvent indicates that an error occurred during processing.
type RunErrorEvent struct {
	ThreadID  string
	Message   string
	ErrorCode string
	Timestamp time.Time
}

func (e RunErrorEvent) EventType() string         { return "RunError" }
func (e RunErrorEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e RunErrorEvent) AggregateID() string       { return e.ThreadID }

// ThreadDeleted is emitted when a thread is deleted.
type ThreadDeleted struct {
	ThreadID  string    `json:"threadId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e ThreadDeleted) EventType() string         { return "ThreadDeleted" }
func (e ThreadDeleted) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadDeleted) AggregateID() string       { return e.ThreadID }

// AgentHandoffEvent is emitted when control is transferred to another agent.
type AgentHandoffEvent struct {
	ThreadID    string
	TargetAgent string
	Timestamp   time.Time
}

func (e AgentHandoffEvent) EventType() string         { return "AgentHandoff" }
func (e AgentHandoffEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e AgentHandoffEvent) AggregateID() string       { return e.ThreadID }

// HumanInputRequestedEvent is emitted when an agent requests input from a human.
type HumanInputRequestedEvent struct {
	ThreadID   string         `json:"threadId"`
	MessageID  string         `json:"messageId"`
	ToolCallID string         `json:"toolCallId"`
	Question   string         `json:"question"`
	Schema     map[string]any `json:"schema"`
	Timestamp  time.Time      `json:"timestamp"`
}

func (e HumanInputRequestedEvent) EventType() string         { return "HumanInputRequested" }
func (e HumanInputRequestedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e HumanInputRequestedEvent) AggregateID() string       { return e.ThreadID }
