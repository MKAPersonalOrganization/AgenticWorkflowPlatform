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
