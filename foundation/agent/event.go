package agent

import "time"

// Event represents something that happened during agent execution.
// Events are streamed back to the caller for real-time updates.
type Event interface {
	// Type returns the event type (e.g., "message", "tool_call", "error").
	Type() EventType

	// Timestamp returns when the event occurred.
	Timestamp() time.Time

	// Data returns the event payload (type-specific).
	Data() any
}

// EventType identifies the kind of event.
type EventType string

const (
	// EventTypeMessage indicates a new message (user or assistant).
	EventTypeMessage EventType = "message"

	// EventTypeToolCall indicates a tool is being invoked.
	EventTypeToolCall EventType = "tool_call"

	// EventTypeToolResult indicates a tool completed.
	EventTypeToolResult EventType = "tool_result"

	// EventTypeThinking indicates the agent is processing.
	EventTypeThinking EventType = "thinking"

	// EventTypeError indicates an error occurred.
	EventTypeError EventType = "error"

	// EventTypeUsage indicates token usage.
	EventTypeUsage EventType = "usage"

	// EventTypeHandoff indicates agent transfer.
	EventTypeHandoff EventType = "handoff"

	// EventTypeTitleGenerated indicates a conversation title was created.
	EventTypeTitleGenerated EventType = "title_generated"

	// EventTypeToolInteraction indicates a tool requires user input or auth.
	EventTypeToolInteraction EventType = "tool_interaction"

	// EventTypeComplete indicates the agent finished successfully.
	EventTypeComplete EventType = "complete"
)

// BaseEvent provides a default Event implementation.
type BaseEvent struct {
	EventType EventType
	EventTime time.Time
	EventData any
}

func (e *BaseEvent) Type() EventType {
	return e.EventType
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.EventTime
}

func (e *BaseEvent) Data() any {
	return e.EventData
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, data any) Event {
	return &BaseEvent{
		EventType: eventType,
		EventTime: time.Now(),
		EventData: data,
	}
}

// MessageEvent is the payload for EventTypeMessage.
type MessageEvent struct {
	Message *Message
}

// ToolCallEvent is the payload for EventTypeToolCall.
type ToolCallEvent struct {
	ToolCall *ToolCall
}

// ToolResultEvent is the payload for EventTypeToolResult.
type ToolResultEvent struct {
	Result *ToolResult
}

// ErrorEvent is the payload for EventTypeError.
type ErrorEvent struct {
	Error string
	Code  string
}

// UsageEvent is the payload for EventTypeUsage.
type UsageEvent struct {
	Usage *Usage
}

// HandoffEvent is the payload for EventTypeHandoff.
type HandoffEvent struct {
	FromAgent string
	ToAgent   string
	Reason    string
}

// TitleEvent is the payload for EventTypeTitleGenerated.
type TitleEvent struct {
	Title string
}

// ToolInteractionEvent is the payload for EventTypeToolInteraction.
type ToolInteractionEvent struct {
	ToolCallID string
	ToolName   string
	Type       string // "input", "auth"
	Prompt     string
	Schema     any
	AuthSpec   any
}
