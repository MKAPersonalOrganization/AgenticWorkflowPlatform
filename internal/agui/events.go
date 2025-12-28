package agui

import (
	"fmt"
	"time"
)

// BaseEvent is the base event for all events in the Agent User Interaction Protocol.
type BaseEvent struct {
	Type      string      `json:"type"`
	Timestamp *int64      `json:"timestamp,omitempty"`
	RawEvent  interface{} `json:"rawEvent,omitempty"`
}

// TextMessageStartEvent represents the start of a text message
type TextMessageStartEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
	Role      string `json:"role,omitempty"`
}

// TextMessageContentEvent represents a chunk of message content
type TextMessageContentEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
	Delta     string `json:"delta"`
}

// TextMessageEndEvent represents the end of a text message
type TextMessageEndEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
}

// Lifecycle Events
//
// These events represent the lifecycle of an agent run. A typical agent run
// follows a predictable pattern: it begins with a RunStarted event, may contain
// multiple optional StepStarted/StepFinished pairs, and concludes with either
// a RunFinished event (success) or a RunError event (failure).
//
// Lifecycle events provide crucial structure to agent runs, enabling frontends to
// track progress, manage UI states appropriately, and handle errors gracefully.
// They create a consistent framework for understanding when operations begin and
// end, making it possible to implement features like loading indicators, progress
// tracking, and error recovery mechanisms.
//
// Event Flow:
//   1. RunStarted (mandatory) - Signals the start of an agent run
//   2. StepStarted/StepFinished (optional, can occur multiple times) - Granular progress tracking
//   3. RunFinished OR RunError (mandatory) - Signals completion or failure
//
// The RunStarted and either RunFinished or RunError events are mandatory,
// forming the boundaries of an agent run. Step events are optional and may occur
// multiple times within a run, allowing for structured, observable progress tracking.

// RunStartedEvent indicates that a run has started.
type RunStartedEvent struct {
	BaseEvent
	ThreadID    string      `json:"threadId"`
	RunID       string      `json:"runId"`
	ParentRunID *string     `json:"parentRunId,omitempty"`
	Input       interface{} `json:"input,omitempty"`
}

// RunFinishedEvent signals the successful completion of an agent run.
//
// The RunFinished event indicates that an agent has successfully completed all its work
// for the current run. Upon receiving this event, frontends should finalize any UI states
// that were waiting on the agent's completion. This event marks a clean termination point
// and indicates that no further processing will occur in this run unless explicitly requested.
// The optional result field can contain any output data produced by the agent run.
//
// Properties:
//   - threadId: ID of the conversation thread
//   - runId: ID of the agent run
//   - result: Optional result data from run
type RunFinishedEvent struct {
	BaseEvent
	ThreadID string      `json:"threadId"`
	RunID    string      `json:"runId"`
	Result   interface{} `json:"result,omitempty"`
}

// RunErrorEvent signals an error during an agent run.
//
// The RunError event indicates that the agent encountered an error it could not recover from,
// causing the run to terminate prematurely. This event provides information about what went wrong,
// allowing frontends to display appropriate error messages and potentially offer recovery options.
// After a RunError event, no further processing will occur in this run.
//
// Properties:
//   - message: Error message
//   - code: Optional error code
type RunErrorEvent struct {
	BaseEvent
	Message string  `json:"message"`
	Code    *string `json:"code,omitempty"`
}

// ToolCallStartEvent signals the start of a tool call
type ToolCallStartEvent struct {
	BaseEvent
	ToolCallID      string  `json:"toolCallId"`
	ToolCallName    string  `json:"toolCallName"`
	ParentMessageID *string `json:"parentMessageId,omitempty"`
}

// ToolCallArgsEvent represents a chunk of tool call arguments
type ToolCallArgsEvent struct {
	BaseEvent
	ToolCallID string `json:"toolCallId"`
	Delta      string `json:"delta"`
}

// ToolCallEndEvent signals the end of a tool call
type ToolCallEndEvent struct {
	BaseEvent
	ToolCallID string `json:"toolCallId"`
}

// ToolCallResultEvent provides the result of a tool call execution
type ToolCallResultEvent struct {
	BaseEvent
	MessageID  string  `json:"messageId"`
	ToolCallID string  `json:"toolCallId"`
	Content    string  `json:"content"`
	Role       *string `json:"role,omitempty"`
}

// StepStartedEvent signals the start of a step within an agent run.
//
// The StepStarted event indicates that the agent is beginning a specific subtask or phase
// of its processing. Steps provide granular visibility into the agent's progress, enabling
// more precise tracking and feedback in the UI. Steps are optional but highly recommended
// for complex operations that benefit from being broken down into observable stages.
// The stepName could be the name of a node or function that is currently executing.
//
// Properties:
//   - stepName: Name of the step
type StepStartedEvent struct {
	BaseEvent
	StepName string `json:"stepName"`
}

// StepFinishedEvent signals the completion of a step within an agent run.
//
// The StepFinished event indicates that the agent has completed a specific subtask or phase.
// When paired with a corresponding StepStarted event, it creates a bounded context for a
// discrete unit of work. Frontends can use these events to update progress indicators, show
// completion animations, or reveal results specific to that step. The stepName must match
// the corresponding StepStarted event to properly pair the beginning and end of the step.
//
// Properties:
//   - stepName: Name of the step
type StepFinishedEvent struct {
	BaseEvent
	StepName string `json:"stepName"`
}

// GenerateMessageID generates a unique message ID
func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// GenerateRunID generates a unique run ID
func GenerateRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}

// GenerateToolCallID generates a unique tool call ID
func GenerateToolCallID() string {
	return fmt.Sprintf("tool_%d", time.Now().UnixNano())
}
