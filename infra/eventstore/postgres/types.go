package postgres

import (
	"time"

	es "aicompass/foundation/eventsourcing"
)

// EventWithPosition wraps a domain event with its global position metadata.
// Used by the projection updater to track catch-up progress.
type EventWithPosition struct {
	GlobalPosition int64
	StreamID       string
	StreamVersion  int64
	EventType      string
	Event          es.Event
	CreatedAt      time.Time
}
