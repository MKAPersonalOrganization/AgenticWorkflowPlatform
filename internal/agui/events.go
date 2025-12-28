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

// RunFinishedEvent represents the successful completion of an agent run
type RunFinishedEvent struct {
	BaseEvent
	ThreadID string      `json:"threadId"`
	RunID    string      `json:"runId"`
	Result   interface{} `json:"result,omitempty"`
}

// RunErrorEvent represents an error during an agent run
type RunErrorEvent struct {
	BaseEvent
	RunID   string  `json:"runId,omitempty"`
	Code    *string `json:"code,omitempty"`
	Message string  `json:"message"`
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
