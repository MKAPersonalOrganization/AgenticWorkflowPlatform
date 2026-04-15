package thread

import (
	"time"
)

// Domain events - transport-agnostic events that represent business domain occurrences.
// These events are emitted by the orchestration service and can be converted to
// transport-specific events (AGUI, A2A, etc.) by the presentation layer.
// All domain events implement the Event interface defined in types.go.

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

// Attachment is a reference to a file stored in the artifact service (image, audio, PDF, etc.).
type Attachment struct {
	URL      string `json:"url"`
	MimeType string `json:"mimeType"`
	FileName string `json:"fileName,omitempty"`
}

// UserMessageAdded is emitted when a user message is added to the thread.
type UserMessageAdded struct {
	ThreadID    string       `json:"threadId"`
	UserID      string       `json:"userId,omitempty"`
	MessageID   string       `json:"messageId"`
	Content     string       `json:"content"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Timestamp   time.Time    `json:"timestamp"`
}

func (e *UserMessageAdded) EventType() string         { return "UserMessageAdded" }
func (e *UserMessageAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e *UserMessageAdded) AggregateID() string       { return e.ThreadID }

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

// ThreadTitleUpdated indicates that a thread's title has been changed.
type ThreadTitleUpdated struct {
	ThreadID  string    `json:"threadId"`
	Title     string    `json:"title"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *ThreadTitleUpdated) EventType() string         { return "ThreadTitleUpdated" }
func (e *ThreadTitleUpdated) EventTimestamp() time.Time { return e.Timestamp }
func (e *ThreadTitleUpdated) AggregateID() string       { return e.ThreadID }

// AssistantMessageStarted is emitted when the assistant begins responding.
type AssistantMessageStarted struct {
	ThreadID  string
	MessageID string
	Timestamp time.Time
}

func (e *AssistantMessageStarted) EventType() string         { return "AssistantMessageStarted" }
func (e *AssistantMessageStarted) EventTimestamp() time.Time { return e.Timestamp }
func (e *AssistantMessageStarted) AggregateID() string       { return e.ThreadID }

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

// ToolNeedsInput is emitted when a tool requires structured user input to continue (elicitation).
// This maps to the SSE event TOOL_NEEDS_INPUT.
type ToolNeedsInput struct {
	ThreadID           string           `json:"threadId"`
	MessageID          string           `json:"messageId"`
	ToolCallID         string           `json:"toolCallId"`
	OriginalToolCallID string           `json:"originalToolCallId"`
	ToolName           string           `json:"toolName"`
	Elicitation        *ElicitationSpec `json:"elicitation"`
	AgentName          string           `json:"agentName"`
	Timestamp          time.Time        `json:"timestamp"`
}

func (e *ToolNeedsInput) EventType() string         { return "ToolNeedsInput" }
func (e *ToolNeedsInput) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolNeedsInput) AggregateID() string       { return e.ThreadID }

// ToolNeedsConfirmation is emitted when a tool requires binary user confirmation (Allow/Deny).
// This maps to the SSE event TOOL_NEEDS_CONFIRMATION.
type ToolNeedsConfirmation struct {
	ThreadID           string    `json:"threadId"`
	MessageID          string    `json:"messageId"`
	ToolCallID         string    `json:"toolCallId"`
	OriginalToolCallID string    `json:"originalToolCallId"`
	ToolName           string    `json:"toolName"`
	Question           string    `json:"question"`
	AgentName          string    `json:"agentName"`
	Timestamp          time.Time `json:"timestamp"`
}

func (e *ToolNeedsConfirmation) EventType() string         { return "ToolNeedsConfirmation" }
func (e *ToolNeedsConfirmation) EventTimestamp() time.Time { return e.Timestamp }
func (e *ToolNeedsConfirmation) AggregateID() string       { return e.ThreadID }

// AgentTransferStarted is emitted when a workflow agent transfers control
// to a different sub-agent. This is a transient event — NOT persisted to the
// event store — used to update the UI with the currently active agent name
// and workflow structure context (step position, branch, iteration).
type AgentTransferStarted struct {
	ThreadID  string `json:"threadId"`
	FromAgent string `json:"fromAgent"`
	ToAgent   string `json:"toAgent"`

	// Workflow structure context — populated when the agent is part of a
	// workflow tree so the UI can render step/branch/iteration indicators.
	WorkflowType  string `json:"workflowType,omitempty"`  // "sequential", "parallel", "loop"
	ParentAgent   string `json:"parentAgent,omitempty"`   // Name of the parent composite agent
	StepIndex     int    `json:"stepIndex,omitempty"`     // 0-based position among siblings
	StepCount     int    `json:"stepCount,omitempty"`     // Total sibling count
	Iteration     int    `json:"iteration,omitempty"`     // 1-based loop iteration
	MaxIterations int    `json:"maxIterations,omitempty"` // Loop max iterations

	Timestamp time.Time `json:"timestamp"`
}

func (e *AgentTransferStarted) EventType() string         { return "AgentTransferStarted" }
func (e *AgentTransferStarted) EventTimestamp() time.Time { return e.Timestamp }
func (e *AgentTransferStarted) AggregateID() string       { return e.ThreadID }

// ThreadDeleted is emitted when a thread is deleted.
type ThreadDeleted struct {
	ThreadID  string    `json:"threadId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *ThreadDeleted) EventType() string         { return "ThreadDeleted" }
func (e *ThreadDeleted) EventTimestamp() time.Time { return e.Timestamp }
func (e *ThreadDeleted) AggregateID() string       { return e.ThreadID }

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

// A2AArtifactReceived is emitted when a remote A2A agent produces a complete
// artifact. Artifacts are distinct named output items (e.g. generated text,
// documents) separate from the streaming text response bubble.
// This is a transient event — NOT persisted to the event store.
type A2AArtifactReceived struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	AgentName  string    `json:"agentName,omitempty"`
	ArtifactID string    `json:"artifactId,omitempty"`
	Title      string    `json:"title,omitempty"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e *A2AArtifactReceived) EventType() string         { return "A2AArtifactReceived" }
func (e *A2AArtifactReceived) EventTimestamp() time.Time { return e.Timestamp }
func (e *A2AArtifactReceived) AggregateID() string       { return e.ThreadID }

// ArtifactVersionSaved is emitted when an agent creates or updates an artifact
// via the ADK artifact service. The frontend uses this to display artifact
// cards with download/preview links and version history.
// This is a transient event — NOT persisted to the event store.
type ArtifactVersionSaved struct {
	ThreadID  string    `json:"threadId"`
	AgentName string    `json:"agentName,omitempty"`
	FileName  string    `json:"fileName"`
	Version   int64     `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *ArtifactVersionSaved) EventType() string         { return "ArtifactVersionSaved" }
func (e *ArtifactVersionSaved) EventTimestamp() time.Time { return e.Timestamp }
func (e *ArtifactVersionSaved) AggregateID() string       { return e.ThreadID }

// StateChangeReported is emitted when an agent sets a value in session state
// (e.g. via Config.OutputKey or a tool's StateDelta). This is a durable
// event — persisted to the event store — enabling stateful agents across
// process restarts and supporting multi-agent workflows that read/write state.
//
// Only user-visible keys are persisted (internal keys prefixed "__" and
// ADK-Go ephemeral "temp:" keys are suppressed).
type StateChangeReported struct {
	ThreadID  string    `json:"threadId"`
	AgentName string    `json:"agentName,omitempty"`
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *StateChangeReported) EventType() string         { return "StateChangeReported" }
func (e *StateChangeReported) EventTimestamp() time.Time { return e.Timestamp }
func (e *StateChangeReported) AggregateID() string       { return e.ThreadID }

// FileAttachmentReceived is emitted when a remote A2A agent produces a file
// attachment (URL or inline data). This is a transient event — NOT persisted.
type FileAttachmentReceived struct {
	ThreadID  string    `json:"threadId"`
	MessageID string    `json:"messageId"`
	AgentName string    `json:"agentName,omitempty"`
	FileURL   string    `json:"fileUrl"`
	FileName  string    `json:"fileName,omitempty"`
	MimeType  string    `json:"mimeType,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *FileAttachmentReceived) EventType() string         { return "FileAttachmentReceived" }
func (e *FileAttachmentReceived) EventTimestamp() time.Time { return e.Timestamp }
func (e *FileAttachmentReceived) AggregateID() string       { return e.ThreadID }

// WorkflowNodeCompleted is emitted when a workflow sub-agent finishes executing.
// Outcome is "success" or "error". DurationMs is the wall-clock time the node
// spent running (from when it received control to when it handed off).
// This is a transient event — NOT persisted to the event store.
type WorkflowNodeCompleted struct {
	ThreadID   string    `json:"threadId"`
	AgentName  string    `json:"agentName"`
	DurationMs int64     `json:"durationMs"`
	Outcome    string    `json:"outcome"` // "success" | "error"
	Timestamp  time.Time `json:"timestamp"`
}

func (e *WorkflowNodeCompleted) EventType() string         { return "WorkflowNodeCompleted" }
func (e *WorkflowNodeCompleted) EventTimestamp() time.Time { return e.Timestamp }
func (e *WorkflowNodeCompleted) AggregateID() string       { return e.ThreadID }

// RunStarted signals the start of an agent run.
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

// RunFinished signals the successful completion of an agent run.
type RunFinished struct {
	ThreadID  string    `json:"threadId"`         // ID of the conversation thread
	RunID     string    `json:"runId"`            // ID of the agent run
	Result    any       `json:"result,omitempty"` // (Optional) Result data from run
	Timestamp time.Time `json:"timestamp"`
}

func (e *RunFinished) EventType() string         { return "RunFinished" }
func (e *RunFinished) EventTimestamp() time.Time { return e.Timestamp }
func (e *RunFinished) AggregateID() string       { return e.ThreadID }

// RunError signals an error during an agent run.
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
