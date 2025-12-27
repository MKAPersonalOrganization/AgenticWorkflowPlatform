package agui

import (
	"fmt"
	"time"
)

// TextMessageStartEvent represents the start of a text message
type TextMessageStartEvent struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	Role      string `json:"role,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// TextMessageContentEvent represents a chunk of message content
type TextMessageContentEvent struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	Delta     string `json:"delta"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// TextMessageEndEvent represents the end of a text message
type TextMessageEndEvent struct {
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// RunStartedEvent represents the start of an agent run
type RunStartedEvent struct {
	Type      string `json:"type"`
	ThreadID  string `json:"threadId"`
	RunID     string `json:"runId"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// RunFinishedEvent represents the successful completion of an agent run
type RunFinishedEvent struct {
	Type      string      `json:"type"`
	ThreadID  string      `json:"threadId"`
	RunID     string      `json:"runId"`
	Result    interface{} `json:"result,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// RunErrorEvent represents an error during an agent run
type RunErrorEvent struct {
	Type      string `json:"type"`
	RunID     string `json:"runId,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// GenerateMessageID generates a unique message ID
func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// GenerateRunID generates a unique run ID
func GenerateRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}
