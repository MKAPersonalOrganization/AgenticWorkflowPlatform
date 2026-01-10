package domain

import "time"

// Domain events - transport-agnostic events that represent business domain occurrences.
// These events are emitted by the orchestration service and can be converted to
// transport-specific events (AGUI, A2A, etc.) by the presentation layer.

// RunStartedEvent indicates that message processing has begun.
type RunStartedEvent struct {
	ThreadID  string
	RunID     string
	Timestamp time.Time
}

func (e RunStartedEvent) EventType() string         { return "run.started" }
func (e RunStartedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e RunStartedEvent) AggregateID() string       { return e.ThreadID }

// RunFinishedEvent indicates that message processing has completed.
type RunFinishedEvent struct {
	ThreadID  string
	RunID     string
	Timestamp time.Time
}

func (e RunFinishedEvent) EventType() string         { return "run.finished" }
func (e RunFinishedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e RunFinishedEvent) AggregateID() string       { return e.ThreadID }

// RunErrorEvent indicates that an error occurred during processing.
type RunErrorEvent struct {
	ThreadID  string
	Message   string
	ErrorCode string
	Timestamp time.Time
}

func (e RunErrorEvent) EventType() string         { return "run.error" }
func (e RunErrorEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e RunErrorEvent) AggregateID() string       { return e.ThreadID }

// ThreadTitleUpdatedEvent indicates that a thread's title has been changed.
type ThreadTitleUpdatedEvent struct {
	ThreadID  string
	Title     string
	Timestamp time.Time
}

func (e ThreadTitleUpdatedEvent) EventType() string         { return "thread.title.updated" }
func (e ThreadTitleUpdatedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ThreadTitleUpdatedEvent) AggregateID() string       { return e.ThreadID }
