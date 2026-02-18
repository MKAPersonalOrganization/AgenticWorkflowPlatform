package postgres

import (
	"time"

	"school-with-ai/domain"
)

// EventWithPosition wraps a domain event with its global position metadata.
// Used by the projection updater to track catch-up progress.
type EventWithPosition struct {
	GlobalPosition int64
	StreamID       string
	StreamVersion  int64
	EventType      string
	Event          domain.Event
	CreatedAt      time.Time
}
