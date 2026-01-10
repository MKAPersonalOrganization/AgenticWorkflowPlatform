package orchestration

// Domain events - transport-agnostic events that represent business domain occurrences.
// These events are emitted by the orchestration service and can be converted to
// transport-specific events (AGUI, A2A, etc.) by the presentation layer.

// RunStartedEvent indicates that message processing has begun.
type RunStartedEvent struct {
	ThreadID string
	RunID    string
}

// RunFinishedEvent indicates that message processing has completed.
type RunFinishedEvent struct {
	ThreadID string
	RunID    string
}

// RunErrorEvent indicates that an error occurred during processing.
type RunErrorEvent struct {
	Message   string
	ErrorCode string
}

// ThreadTitleUpdatedEvent indicates that a thread's title has been changed.
type ThreadTitleUpdatedEvent struct {
	ThreadID string
	Title    string
}
