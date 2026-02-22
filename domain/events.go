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

func (e *ThreadCreated) EventType() string         { return "ThreadCreated" }
func (e *ThreadCreated) EventTimestamp() time.Time { return e.Timestamp }
func (e *ThreadCreated) AggregateID() string       { return e.ThreadID }
func (e *ThreadCreated) sealed()                   {}

// UserMessageAdded is emitted when a user message is added to the thread.
type UserMessageAdded struct {
	ThreadID  string    `json:"threadId"`
	UserID    string    `json:"userId,omitempty"`
	MessageID string    `json:"messageId"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *UserMessageAdded) EventType() string         { return "UserMessageAdded" }
func (e *UserMessageAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e *UserMessageAdded) AggregateID() string       { return e.ThreadID }
func (e *UserMessageAdded) sealed()                   {}

// AssistantMessageAdded is emitted when an assistant message is added to the thread.
type AssistantMessageAdded struct {
	ThreadID  string     `json:"threadId"`
	MessageID string     `json:"messageId"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	AgentName string     `json:"agentName,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

func (e *AssistantMessageAdded) EventType() string         { return "AssistantMessageAdded" }
func (e *AssistantMessageAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e *AssistantMessageAdded) AggregateID() string       { return e.ThreadID }
func (e *AssistantMessageAdded) sealed()                   {}

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

func (e *ToolResultAdded) EventType() string         { return "ToolResultAdded" }
func (e *ToolResultAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolResultAdded) AggregateID() string       { return e.ThreadID }
func (e *ToolResultAdded) sealed()                   {}

// ThreadTitleUpdated indicates that a thread's title has been changed.
type ThreadTitleUpdated struct {
	ThreadID  string    `json:"threadId"`
	Title     string    `json:"title"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *ThreadTitleUpdated) EventType() string         { return "ThreadTitleUpdated" }
func (e *ThreadTitleUpdated) EventTimestamp() time.Time { return e.Timestamp }
func (e *ThreadTitleUpdated) AggregateID() string       { return e.ThreadID }
func (e *ThreadTitleUpdated) sealed()                   {}

// AssistantMessageStarted is emitted when the assistant begins responding.
type AssistantMessageStarted struct {
	ThreadID  string
	MessageID string
	Timestamp time.Time
}

func (e *AssistantMessageStarted) EventType() string         { return "AssistantMessageStarted" }
func (e *AssistantMessageStarted) EventTimestamp() time.Time { return e.Timestamp }
func (e *AssistantMessageStarted) AggregateID() string       { return e.ThreadID }
func (e *AssistantMessageStarted) sealed()                   {}

// AssistantContentStreamed is emitted for each content delta during streaming.
type AssistantContentStreamed struct {
	ThreadID  string
	MessageID string
	Delta     string
	Timestamp time.Time
}

func (e *AssistantContentStreamed) EventType() string         { return "AssistantContentStreamed" }
func (e *AssistantContentStreamed) EventTimestamp() time.Time { return e.Timestamp }
func (e *AssistantContentStreamed) AggregateID() string       { return e.ThreadID }
func (e *AssistantContentStreamed) sealed()                   {}

// AssistantMessageCompleted is emitted when the assistant completes a message.
type AssistantMessageCompleted struct {
	ThreadID  string
	MessageID string
	Content   string
	AgentName string
	ToolCalls []ToolCall
	Timestamp time.Time
}

func (e *AssistantMessageCompleted) EventType() string         { return "AssistantMessageCompleted" }
func (e *AssistantMessageCompleted) EventTimestamp() time.Time { return e.Timestamp }
func (e *AssistantMessageCompleted) AggregateID() string       { return e.ThreadID }
func (e *AssistantMessageCompleted) sealed()                   {}

// ToolStart is emitted when a tool execution began (UI feedback only).
// This is a transient event - NOT persisted to event store.
type ToolStart struct {
	ThreadID      string    `json:"threadId"`
	MessageID     string    `json:"messageId"`
	ToolCallID    string    `json:"toolCallId"`
	ToolName      string    `json:"toolName"`
	ToolArguments string    `json:"arguments,omitempty"`
	AgentName     string    `json:"agentName"`
	Timestamp     time.Time `json:"timestamp"`
}

func (e *ToolStart) EventType() string         { return "ToolStart" }
func (e *ToolStart) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolStart) AggregateID() string       { return e.ThreadID }
func (e *ToolStart) sealed()                   {}

// ToolContentStreamed is emitted for each content delta during streaming tool execution.
// This is a transient event - NOT persisted to event store.
type ToolContentStreamed struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	ToolName   string    `json:"toolName"`
	Delta      string    `json:"delta"`
	Field      string    `json:"field,omitempty"` // Which part of the result is streaming (e.g. "answer", "thoughts")
	Timestamp  time.Time `json:"timestamp"`
}

func (e *ToolContentStreamed) EventType() string         { return "ToolContentStreamed" }
func (e *ToolContentStreamed) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolContentStreamed) AggregateID() string       { return e.ThreadID }
func (e *ToolContentStreamed) sealed()                   {}

// ThreadDeleted is emitted when a thread is deleted.
type ThreadDeleted struct {
	ThreadID  string    `json:"threadId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *ThreadDeleted) EventType() string         { return "ThreadDeleted" }
func (e *ThreadDeleted) EventTimestamp() time.Time { return e.Timestamp }
func (e *ThreadDeleted) AggregateID() string       { return e.ThreadID }
func (e *ThreadDeleted) sealed()                   {}

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

func (e *TokenUsageReported) EventType() string         { return "TokenUsageReported" }
func (e *TokenUsageReported) EventTimestamp() time.Time { return e.Timestamp }
func (e *TokenUsageReported) AggregateID() string       { return e.ThreadID }
func (e *TokenUsageReported) sealed()                   {}

// ToolNeedsInput is emitted when a tool requires user input to continue (elicitation).
type ToolNeedsInput struct {
	ThreadID    string           `json:"threadId"`
	MessageID   string           `json:"messageId"`
	ToolCallID  string           `json:"toolCallId"`
	ToolName    string           `json:"toolName"`
	Elicitation *ElicitationSpec `json:"elicitation"`
	AgentName   string           `json:"agentName"`
	Timestamp   time.Time        `json:"timestamp"`
}

func (e *ToolNeedsInput) EventType() string         { return "ToolNeedsInput" }
func (e *ToolNeedsInput) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolNeedsInput) AggregateID() string       { return e.ThreadID }
func (e *ToolNeedsInput) sealed()                   {}

// ToolNeedsAuth is emitted when a tool requires user authorization to continue.
type ToolNeedsAuth struct {
	ThreadID   string             `json:"threadId"`
	MessageID  string             `json:"messageId"`
	ToolCallID string             `json:"toolCallId"`
	ToolName   string             `json:"toolName"`
	AuthSpec   *AuthorizationSpec `json:"authSpec"`
	AgentName  string             `json:"agentName"`
	Timestamp  time.Time          `json:"timestamp"`
}

func (e *ToolNeedsAuth) EventType() string         { return "ToolNeedsAuth" }
func (e *ToolNeedsAuth) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolNeedsAuth) AggregateID() string       { return e.ThreadID }
func (e *ToolNeedsAuth) sealed()                   {}

// InputProvided is emitted when user provided input to a waiting tool.
type InputProvided struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	Input      string    `json:"input"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e *InputProvided) EventType() string         { return "InputProvided" }
func (e *InputProvided) EventTimestamp() time.Time { return e.Timestamp }
func (e *InputProvided) AggregateID() string       { return e.ThreadID }
func (e *InputProvided) sealed()                   {}

// AuthGranted is emitted when user granted authorization to a waiting tool.
type AuthGranted struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e *AuthGranted) EventType() string         { return "AuthGranted" }
func (e *AuthGranted) EventTimestamp() time.Time { return e.Timestamp }
func (e *AuthGranted) AggregateID() string       { return e.ThreadID }
func (e *AuthGranted) sealed()                   {}

// AuthDenied is emitted when user denied authorization to a waiting tool.
type AuthDenied struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e *AuthDenied) EventType() string         { return "AuthDenied" }
func (e *AuthDenied) EventTimestamp() time.Time { return e.Timestamp }
func (e *AuthDenied) AggregateID() string       { return e.ThreadID }
func (e *AuthDenied) sealed()                   {}

// ToolCancelled is emitted when tool execution was cancelled.
type ToolCancelled struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	ToolName   string    `json:"toolName"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e *ToolCancelled) EventType() string         { return "ToolCancelled" }
func (e *ToolCancelled) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolCancelled) AggregateID() string       { return e.ThreadID }
func (e *ToolCancelled) sealed()                   {}

// RunStarted signals the start of an agent run.
// The RunStarted event is the first event emitted when an agent begins processing a request.
// It establishes a new execution context identified by a unique runId. This event serves as
// a marker for frontends to initialize UI elements such as progress indicators or loading states.
// It also provides crucial identifiers that can be used to associate subsequent events with this specific run.
type RunStarted struct {
	ThreadID    string    `json:"threadId"`              // ID of the conversation thread
	RunID       string    `json:"runId"`                 // ID of the agent run
	ParentRunID string    `json:"parentRunId,omitempty"` // (Optional) Lineage pointer for branching/time travel
	Input       any       `json:"input,omitempty"`       // (Optional) The exact agent input payload sent to the agent
	Timestamp   time.Time `json:"timestamp"`
}

func (e *RunStarted) EventType() string         { return "RunStarted" }
func (e *RunStarted) EventTimestamp() time.Time { return e.Timestamp }
func (e *RunStarted) AggregateID() string       { return e.ThreadID }
func (e *RunStarted) sealed()                   {}

// RunFinished signals the successful completion of an agent run.
// The RunFinished event indicates that an agent has successfully completed all its work for the current run.
// Upon receiving this event, frontends should finalize any UI states that were waiting on the agent's completion.
// This event marks a clean termination point and indicates that no further processing will occur in this run
// unless explicitly requested. The optional result field can contain any output data produced by the agent run.
type RunFinished struct {
	ThreadID  string    `json:"threadId"`         // ID of the conversation thread
	RunID     string    `json:"runId"`            // ID of the agent run
	Result    any       `json:"result,omitempty"` // (Optional) Result data from run
	Timestamp time.Time `json:"timestamp"`
}

func (e *RunFinished) EventType() string         { return "RunFinished" }
func (e *RunFinished) EventTimestamp() time.Time { return e.Timestamp }
func (e *RunFinished) AggregateID() string       { return e.ThreadID }
func (e *RunFinished) sealed()                   {}

// RunError signals an error during an agent run.
// The RunError event indicates that the agent encountered an error it could not recover from,
// causing the run to terminate prematurely. This event provides information about what went wrong,
// allowing frontends to display appropriate error messages and potentially offer recovery options.
// After a RunError event, no further processing will occur in this run.
type RunError struct {
	ThreadID  string    `json:"threadId"`       // ID of the conversation thread
	RunID     string    `json:"runId"`          // ID of the agent run
	Message   string    `json:"message"`        // Error message
	Code      string    `json:"code,omitempty"` // (Optional) Error code
	Timestamp time.Time `json:"timestamp"`
}

func (e *RunError) EventType() string         { return "RunError" }
func (e *RunError) EventTimestamp() time.Time { return e.Timestamp }
func (e *RunError) AggregateID() string       { return e.ThreadID }
func (e *RunError) sealed()                   {}
