package agent

// MessageStatus indicates the current state of a message.
type MessageStatus string

const (
	// StatusInProgress indicates the message is being generated.
	StatusInProgress MessageStatus = "in_progress"

	// StatusCompleted indicates the message is fully generated.
	StatusCompleted MessageStatus = "completed"
)

// StreamingMode defines how events are streamed during agent execution.
type StreamingMode string

const (
	// StreamingModeNone disables streaming; return only final results.
	StreamingModeNone StreamingMode = "none"

	// StreamingModeSSE enables server-sent events streaming.
	// LLM response parts are streamed as they're generated.
	StreamingModeSSE StreamingMode = "sse"
)
