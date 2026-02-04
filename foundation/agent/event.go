package agent

import "time"

// Event represents something that happened during agent execution.
// Events are streamed back to the caller for real-time updates.
//
// Per ADR 003 and ADR 004:
// - Events are classified as Ephemeral or Persistent
// - All event types use pointer receivers
type Event interface {
	// Type returns the event type (e.g., "message", "tool_call", "error").
	Type() EventType

	// Timestamp returns when the event occurred.
	Timestamp() time.Time

	// Data returns the event payload (type-specific).
	Data() any
}

// EphemeralEvent marks an event as transient (not persisted).
// Per ADR 003: Ephemeral events are for real-time signaling only.
// Examples: streaming deltas, progress indicators, run lifecycle.
type EphemeralEvent interface {
	Event
	ephemeral() // Marker method
}

// PersistentEvent marks an event as durable (persisted to event store).
// Per ADR 003: Persistent events represent domain state changes.
// Examples: MessageAdded, ToolResultAdded, ThreadCreated.
type PersistentEvent interface {
	Event
	persistent() // Marker method
}

// EventType identifies the kind of event.
type EventType string

const (
	// EventTypeMessage indicates a new message (user or assistant).
	EventTypeMessage EventType = "message"

	// EventTypeMessageDelta indicates streaming message content.
	EventTypeMessageDelta EventType = "message_delta"

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

	// EventTypeStateChange indicates session state was modified.
	EventTypeStateChange EventType = "state_change"
)

// BaseEvent provides a default Event implementation.
// Use this for simple event types or when creating events dynamically.
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

// MessageEvent is emitted when a complete message is available.
// This is a PERSISTENT event - it represents a domain state change.
type MessageEvent struct {
	Message   *Message
	EventTime time.Time
}

func (e *MessageEvent) Type() EventType      { return EventTypeMessage }
func (e *MessageEvent) Timestamp() time.Time { return e.EventTime }
func (e *MessageEvent) Data() any            { return e.Message }
func (e *MessageEvent) persistent()          {}

// MessageDeltaEvent is emitted during streaming for partial content.
// This is an EPHEMERAL event - for real-time UI updates only.
type MessageDeltaEvent struct {
	MessageID string
	Delta     string
	EventTime time.Time
}

func (e *MessageDeltaEvent) Type() EventType      { return EventTypeMessageDelta }
func (e *MessageDeltaEvent) Timestamp() time.Time { return e.EventTime }
func (e *MessageDeltaEvent) Data() any            { return e.Delta }
func (e *MessageDeltaEvent) ephemeral()           {}

// ToolCallEvent is emitted when a tool execution starts.
// This is an EPHEMERAL event - the persistent record is ToolResultEvent.
type ToolCallEvent struct {
	ToolCall  *ToolCall
	AgentName string
	EventTime time.Time
}

func (e *ToolCallEvent) Type() EventType      { return EventTypeToolCall }
func (e *ToolCallEvent) Timestamp() time.Time { return e.EventTime }
func (e *ToolCallEvent) Data() any            { return e.ToolCall }
func (e *ToolCallEvent) ephemeral()           {}

// ToolResultEvent is emitted when a tool execution completes.
// This is a PERSISTENT event - it represents a domain state change.
type ToolResultEvent struct {
	Result    *ToolResult
	AgentName string
	EventTime time.Time
}

func (e *ToolResultEvent) Type() EventType      { return EventTypeToolResult }
func (e *ToolResultEvent) Timestamp() time.Time { return e.EventTime }
func (e *ToolResultEvent) Data() any            { return e.Result }
func (e *ToolResultEvent) persistent()          {}

// ErrorEvent is emitted when an error occurs.
// This is an EPHEMERAL event - errors may be logged but not persisted to event store.
type ErrorEvent struct {
	Error     string
	Code      string
	EventTime time.Time
}

func (e *ErrorEvent) Type() EventType      { return EventTypeError }
func (e *ErrorEvent) Timestamp() time.Time { return e.EventTime }
func (e *ErrorEvent) Data() any            { return e.Error }
func (e *ErrorEvent) ephemeral()           {}

// UsageEvent is emitted with token consumption metrics.
// This is a PERSISTENT event - usage tracking is a business requirement.
type UsageEvent struct {
	Usage     *Usage
	EventTime time.Time
}

func (e *UsageEvent) Type() EventType      { return EventTypeUsage }
func (e *UsageEvent) Timestamp() time.Time { return e.EventTime }
func (e *UsageEvent) Data() any            { return e.Usage }
func (e *UsageEvent) persistent()          {}

// HandoffEvent is emitted when control transfers between agents.
// This is an EPHEMERAL event - handoffs are execution flow, not domain state.
type HandoffEvent struct {
	FromAgent string
	ToAgent   string
	Reason    string
	EventTime time.Time
}

func (e *HandoffEvent) Type() EventType      { return EventTypeHandoff }
func (e *HandoffEvent) Timestamp() time.Time { return e.EventTime }
func (e *HandoffEvent) Data() any            { return e }
func (e *HandoffEvent) ephemeral()           {}

// TitleEvent is emitted when a conversation title is generated.
// This is a PERSISTENT event - title is part of thread state.
type TitleEvent struct {
	Title     string
	EventTime time.Time
}

func (e *TitleEvent) Type() EventType      { return EventTypeTitleGenerated }
func (e *TitleEvent) Timestamp() time.Time { return e.EventTime }
func (e *TitleEvent) Data() any            { return e.Title }
func (e *TitleEvent) persistent()          {}

// ToolInteractionEvent is emitted when a tool requires user input or auth.
// This is a PERSISTENT event - interaction state must survive restarts.
type ToolInteractionEvent struct {
	ToolCallID      string
	ToolName        string
	InteractionType string // "input", "auth", "confirmation"
	Prompt          string
	Schema          any
	AuthSpec        any
	EventTime       time.Time
}

func (e *ToolInteractionEvent) Type() EventType      { return EventTypeToolInteraction }
func (e *ToolInteractionEvent) Timestamp() time.Time { return e.EventTime }
func (e *ToolInteractionEvent) Data() any            { return e }
func (e *ToolInteractionEvent) persistent()          {}

// CompleteEvent is emitted when the agent finishes processing.
// This is an EPHEMERAL event - completion is signaling, not state.
type CompleteEvent struct {
	FinalMessage *Message
	EventTime    time.Time
}

func (e *CompleteEvent) Type() EventType      { return EventTypeComplete }
func (e *CompleteEvent) Timestamp() time.Time { return e.EventTime }
func (e *CompleteEvent) Data() any            { return e.FinalMessage }
func (e *CompleteEvent) ephemeral()           {}

// StateChangeEvent is emitted when session state is modified.
// This is a PERSISTENT event - state changes are domain facts.
type StateChangeEvent struct {
	Key       string
	Value     any
	EventTime time.Time
}

func (e *StateChangeEvent) Type() EventType      { return EventTypeStateChange }
func (e *StateChangeEvent) Timestamp() time.Time { return e.EventTime }
func (e *StateChangeEvent) Data() any            { return map[string]any{e.Key: e.Value} }
func (e *StateChangeEvent) persistent()          {}

// Verify interface implementations at compile time
var (
	_ PersistentEvent = (*MessageEvent)(nil)
	_ PersistentEvent = (*ToolResultEvent)(nil)
	_ PersistentEvent = (*UsageEvent)(nil)
	_ PersistentEvent = (*TitleEvent)(nil)
	_ PersistentEvent = (*ToolInteractionEvent)(nil)
	_ PersistentEvent = (*StateChangeEvent)(nil)

	_ EphemeralEvent = (*MessageDeltaEvent)(nil)
	_ EphemeralEvent = (*ToolCallEvent)(nil)
	_ EphemeralEvent = (*ErrorEvent)(nil)
	_ EphemeralEvent = (*HandoffEvent)(nil)
	_ EphemeralEvent = (*CompleteEvent)(nil)
)
