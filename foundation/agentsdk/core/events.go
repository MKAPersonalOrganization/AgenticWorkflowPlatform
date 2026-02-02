package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateMessageID generates a unique message ID.
func GenerateMessageID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random ID: %s", err))
	}
	return hex.EncodeToString(b)
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

// ToolCall represents a tool/function call made by the assistant.
type ToolCall struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // "function"
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// Event is the base interface for all SDK events.
type Event interface {
	EventType() string
	EventTimestamp() time.Time
	AggregateID() string // ThreadID
}

// BaseEvent provides common fields for all events.
type BaseEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	ThreadID  string    `json:"threadId"`
}

func (e BaseEvent) EventType() string         { return e.Type }
func (e BaseEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e BaseEvent) AggregateID() string       { return e.ThreadID }

// -- SDK Events --

// -- SDK Events --

// AssistantMessageStarted is emitted when the assistant begins responding.
type AssistantMessageStarted struct {
	ThreadID  string
	MessageID string
	Timestamp time.Time
}

func (e AssistantMessageStarted) EventType() string         { return "AssistantMessageStarted" }
func (e AssistantMessageStarted) EventTimestamp() time.Time { return e.Timestamp }
func (e AssistantMessageStarted) AggregateID() string       { return e.ThreadID }

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

	// Detailed breakdown
	CachedTokens             int `json:"cachedTokens,omitempty"`
	AudioInputTokens         int `json:"audioInputTokens,omitempty"`
	ReasoningTokens          int `json:"reasoningTokens,omitempty"`
	AudioOutputTokens        int `json:"audioOutputTokens,omitempty"`
	AcceptedPredictionTokens int `json:"acceptedPredictionTokens,omitempty"`
	RejectedPredictionTokens int `json:"rejectedPredictionTokens,omitempty"`

	Timestamp time.Time `json:"timestamp"`
}

func (e TokenUsageReported) EventType() string         { return "TokenUsageReported" }
func (e TokenUsageReported) EventTimestamp() time.Time { return e.Timestamp }
func (e TokenUsageReported) AggregateID() string       { return e.ThreadID }

// ToolStart is emitted when a tool begins execution.
type ToolStart struct {
	ThreadID      string    `json:"threadId"`
	MessageID     string    `json:"messageId"`
	ToolCallID    string    `json:"toolCallId"`
	ToolName      string    `json:"toolName"`
	ToolArguments string    `json:"arguments,omitempty"`
	AgentName     string    `json:"agentName"`
	Timestamp     time.Time `json:"timestamp"`
}

func (e ToolStart) EventType() string         { return "ToolStart" }
func (e ToolStart) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolStart) AggregateID() string       { return e.ThreadID }

// ToolNeedsInput is emitted when a tool requests user input (elicitation).
type ToolNeedsInput struct {
	ThreadID    string           `json:"threadId"`
	MessageID   string           `json:"messageId"`
	ToolCallID  string           `json:"toolCallId"`
	ToolName    string           `json:"toolName"`
	Elicitation *ElicitationSpec `json:"elicitation"`
	AgentName   string           `json:"agentName"`
	Timestamp   time.Time        `json:"timestamp"`
}

func (e ToolNeedsInput) EventType() string         { return "ToolNeedsInput" }
func (e ToolNeedsInput) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolNeedsInput) AggregateID() string       { return e.ThreadID }

// ToolNeedsAuth is emitted when a tool requests user authorization.
type ToolNeedsAuth struct {
	ThreadID   string             `json:"threadId"`
	MessageID  string             `json:"messageId"`
	ToolCallID string             `json:"toolCallId"`
	ToolName   string             `json:"toolName"`
	AuthSpec   *AuthorizationSpec `json:"authSpec"`
	AgentName  string             `json:"agentName"`
	Timestamp  time.Time          `json:"timestamp"`
}

func (e ToolNeedsAuth) EventType() string         { return "ToolNeedsAuth" }
func (e ToolNeedsAuth) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolNeedsAuth) AggregateID() string       { return e.ThreadID }

// InputProvided is emitted when user provides input to a waiting tool.
type InputProvided struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	Input      string    `json:"input"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e InputProvided) EventType() string         { return "InputProvided" }
func (e InputProvided) EventTimestamp() time.Time { return e.Timestamp }
func (e InputProvided) AggregateID() string       { return e.ThreadID }

// AuthGranted is emitted when user grants authorization to a waiting tool.
type AuthGranted struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e AuthGranted) EventType() string         { return "AuthGranted" }
func (e AuthGranted) EventTimestamp() time.Time { return e.Timestamp }
func (e AuthGranted) AggregateID() string       { return e.ThreadID }

// AuthDenied is emitted when user denies authorization to a waiting tool.
type AuthDenied struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e AuthDenied) EventType() string         { return "AuthDenied" }
func (e AuthDenied) EventTimestamp() time.Time { return e.Timestamp }
func (e AuthDenied) AggregateID() string       { return e.ThreadID }

// ToolCancelled is emitted when a tool execution is cancelled.
type ToolCancelled struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	ToolName   string    `json:"toolName"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e ToolCancelled) EventType() string         { return "ToolCancelled" }
func (e ToolCancelled) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolCancelled) AggregateID() string       { return e.ThreadID }

// ActionsDetected is emitted when the engine identifies tool calls that need execution.
type ActionsDetected struct {
	ThreadID  string     `json:"threadId"`
	MessageID string     `json:"messageId"`
	ToolCalls []ToolCall `json:"toolCalls"`
	Timestamp time.Time  `json:"timestamp"`
}

func (e ActionsDetected) EventType() string         { return "ActionsDetected" }
func (e ActionsDetected) EventTimestamp() time.Time { return e.Timestamp }
func (e ActionsDetected) AggregateID() string       { return e.ThreadID }

// TurnFinished is emitted when a turn completes.
type TurnFinished struct {
	ThreadID  string    `json:"threadId"`
	MessageID string    `json:"messageId"`
	Timestamp time.Time `json:"timestamp"`
}

func (e TurnFinished) EventType() string         { return "TurnFinished" }
func (e TurnFinished) EventTimestamp() time.Time { return e.Timestamp }
func (e TurnFinished) AggregateID() string       { return e.ThreadID }

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

// AssistantMessageAdded is emitted when an assistant message is added to the thread.
// This is the durable event version that includes complete message data.
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

// ToolResultAdded is emitted when a tool execution result is added.
type ToolResultAdded struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	ToolName   string    `json:"toolName"`
	Result     string    `json:"result"`
	IsError    bool      `json:"isError"`
	AgentName  string    `json:"agentName,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e ToolResultAdded) EventType() string         { return "ToolResultAdded" }
func (e ToolResultAdded) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolResultAdded) AggregateID() string       { return e.ThreadID }

// EngineStateChanged is emitted when the engine state changes.
type EngineStateChanged struct {
	ThreadID  string                 `json:"threadId"`
	State     string                 `json:"state"`
	AgentName string                 `json:"agentName,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (e EngineStateChanged) EventType() string         { return "EngineStateChanged" }
func (e EngineStateChanged) EventTimestamp() time.Time { return e.Timestamp }
func (e EngineStateChanged) AggregateID() string       { return e.ThreadID }
