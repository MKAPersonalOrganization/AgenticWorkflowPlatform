package domain

import (
	"time"
)

// Domain events - transport-agnostic events that represent business domain occurrences.
// These events are emitted by the orchestration service and can be converted to
// transport-specific events (AGUI, A2A, etc.) by the presentation layer.
// All domain events implement the Event interface defined in domain/domain.go.

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
func (e ThreadCreated) sealed()                   {}

// UserMessageAdded is emitted when a user message is added to the thread.
type UserMessageAdded struct {
	ThreadID  string    `json:"threadId"`
	UserID    string    `json:"userId,omitempty"`
	MessageID string    `json:"messageId"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func (e UserMessageAdded) EventType() string         { return "UserMessageAdded" }
func (e UserMessageAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e UserMessageAdded) AggregateID() string       { return e.ThreadID }
func (e UserMessageAdded) sealed()                   {}

// AssistantMessageAdded is emitted when an assistant message is added to the thread.
type AssistantMessageAdded struct {
	ThreadID  string     `json:"threadId"`
	MessageID string     `json:"messageId"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	AgentName string     `json:"agentName,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

func (e AssistantMessageAdded) EventType() string         { return "AssistantMessageAdded" }
func (e AssistantMessageAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantMessageAdded) AggregateID() string       { return e.ThreadID }
func (e AssistantMessageAdded) sealed()                   {}

// ToolResultAdded is emitted when a tool execution result is added to the thread.
type ToolResultAdded struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	ToolName   string    `json:"toolName,omitempty"`
	Result     string    `json:"result"`
	IsError    bool      `json:"isError"`
	AgentName  string    `json:"agentName,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e ToolResultAdded) EventType() string         { return "ToolResultAdded" }
func (e ToolResultAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolResultAdded) AggregateID() string       { return e.ThreadID }
func (e ToolResultAdded) sealed()                   {}

// ThreadTitleUpdated indicates that a thread's title has been changed.
type ThreadTitleUpdated struct {
	ThreadID  string    `json:"threadId"`
	Title     string    `json:"title"`
	Timestamp time.Time `json:"timestamp"`
}

func (e ThreadTitleUpdated) EventType() string         { return "ThreadTitleUpdated" }
func (e ThreadTitleUpdated) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadTitleUpdated) AggregateID() string       { return e.ThreadID }
func (e ThreadTitleUpdated) sealed()                   {}

// AssistantMessageStarted is emitted when the assistant begins responding.
type AssistantMessageStarted struct {
	ThreadID  string
	MessageID string
	Timestamp time.Time
}

func (e AssistantMessageStarted) EventType() string         { return "AssistantMessageStarted" }
func (e AssistantMessageStarted) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantMessageStarted) AggregateID() string       { return e.ThreadID }
func (e AssistantMessageStarted) sealed()                   {}

// AssistantContentStreamed is emitted for each content delta during streaming.
type AssistantContentStreamed struct {
	ThreadID  string
	MessageID string
	Delta     string
	Timestamp time.Time
}

func (e AssistantContentStreamed) EventType() string         { return "AssistantContentStreamed" }
func (e AssistantContentStreamed) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantContentStreamed) AggregateID() string       { return e.ThreadID }
func (e AssistantContentStreamed) sealed()                   {}

// AssistantMessageCompleted is emitted when the assistant completes a message.
type AssistantMessageCompleted struct {
	ThreadID  string
	MessageID string
	Content   string
	AgentName string
	ToolCalls []ToolCall
	Timestamp time.Time
}

func (e AssistantMessageCompleted) EventType() string         { return "AssistantMessageCompleted" }
func (e AssistantMessageCompleted) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantMessageCompleted) AggregateID() string       { return e.ThreadID }
func (e AssistantMessageCompleted) sealed()                   {}

// RunStarted indicates that message processing has begun.
type RunStarted struct {
	ThreadID  string
	RunID     string
	Timestamp time.Time
}

func (e RunStarted) EventType() string         { return "RunStarted" }
func (e RunStarted) EventTimestamp() time.Time { return e.Timestamp }
func (e RunStarted) AggregateID() string       { return e.ThreadID }
func (e RunStarted) sealed()                   {}

// RunFinished indicates that message processing has completed.
type RunFinished struct {
	ThreadID  string
	RunID     string
	Timestamp time.Time
}

func (e RunFinished) EventType() string         { return "RunFinished" }
func (e RunFinished) EventTimestamp() time.Time { return e.Timestamp }
func (e RunFinished) AggregateID() string       { return e.ThreadID }
func (e RunFinished) sealed()                   {}

// RunError indicates that an error occurred during processing.
type RunError struct {
	ThreadID  string
	Message   string
	ErrorCode string
	Timestamp time.Time
}

func (e RunError) EventType() string         { return "RunError" }
func (e RunError) EventTimestamp() time.Time { return e.Timestamp }
func (e RunError) AggregateID() string       { return e.ThreadID }
func (e RunError) sealed()                   {}

// ThreadDeleted is emitted when a thread is deleted.
type ThreadDeleted struct {
	ThreadID  string    `json:"threadId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e ThreadDeleted) EventType() string         { return "ThreadDeleted" }
func (e ThreadDeleted) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadDeleted) AggregateID() string       { return e.ThreadID }
func (e ThreadDeleted) sealed()                   {}

// TokenUsageReported is emitted when token usage information is available.
type TokenUsageReported struct {
	ThreadID  string `json:"threadId"`
	MessageID string `json:"messageId"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`

	// Basic token counts
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`

	// Detailed prompt token breakdown
	CachedTokens     int `json:"cachedTokens,omitempty"`
	AudioInputTokens int `json:"audioInputTokens,omitempty"`

	// Detailed completion token breakdown
	ReasoningTokens          int `json:"reasoningTokens,omitempty"`
	AudioOutputTokens        int `json:"audioOutputTokens,omitempty"`
	AcceptedPredictionTokens int `json:"acceptedPredictionTokens,omitempty"`
	RejectedPredictionTokens int `json:"rejectedPredictionTokens,omitempty"`

	Timestamp time.Time `json:"timestamp"`
}

func (e TokenUsageReported) EventType() string         { return "TokenUsageReported" }
func (e TokenUsageReported) EventTimestamp() time.Time { return e.Timestamp }
func (e TokenUsageReported) AggregateID() string       { return e.ThreadID }
func (e TokenUsageReported) sealed()                   {}

// EngineStateChanged is emitted when the engine's high-level state changes.
type EngineStateChanged struct {
	ThreadID  string    `json:"threadId"`
	NewState  string    `json:"newState"`
	Timestamp time.Time `json:"timestamp"`
}

func (e EngineStateChanged) EventType() string         { return "EngineStateChanged" }
func (e EngineStateChanged) EventTimestamp() time.Time { return e.Timestamp }
func (e EngineStateChanged) AggregateID() string       { return e.ThreadID }
func (e EngineStateChanged) sealed()                   {}

// ActionsDetected is emitted when the engine identifies tool calls that need execution.
// NOTE: This is a transient informational event - NOT persisted to event store.
// It's used for real-time UI updates only and does not modify aggregate state.
type ActionsDetected struct {
	ThreadID  string     `json:"threadId"`
	MessageID string     `json:"messageId"`
	ToolCalls []ToolCall `json:"toolCalls"`
	Timestamp time.Time  `json:"timestamp"`
}

func (e ActionsDetected) EventType() string         { return "ActionsDetected" }
func (e ActionsDetected) EventTimestamp() time.Time { return e.Timestamp }
func (e ActionsDetected) AggregateID() string       { return e.ThreadID }
func (e ActionsDetected) sealed()                   {}

// TurnFinished is emitted when a turn completes and no further actions are required from the engine.
// NOTE: This is a transient informational event - NOT persisted to event store.
// It's used for real-time UI updates only and does not modify aggregate state.
type TurnFinished struct {
	ThreadID  string    `json:"threadId"`
	MessageID string    `json:"messageId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e TurnFinished) EventType() string         { return "TurnFinished" }
func (e TurnFinished) EventTimestamp() time.Time { return e.Timestamp }
func (e TurnFinished) AggregateID() string       { return e.ThreadID }
func (e TurnFinished) sealed()                   {}

// ExecutionStateReported is emitted to persist the engine's internal safety and loop detection state.
type ExecutionStateReported struct {
	ThreadID     string    `json:"threadId"`
	Iteration    int       `json:"iteration"`
	Total        int       `json:"total"`
	LoopStateHex string    `json:"loopStateHex,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

func (e ExecutionStateReported) EventType() string         { return "ExecutionStateReported" }
func (e ExecutionStateReported) EventTimestamp() time.Time { return e.Timestamp }
func (e ExecutionStateReported) AggregateID() string       { return e.ThreadID }
func (e ExecutionStateReported) sealed()                   {}
