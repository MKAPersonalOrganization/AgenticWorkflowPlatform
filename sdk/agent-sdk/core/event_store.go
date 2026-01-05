package sdk

import (
	"context"
)

// EventStore is the source of truth in the event sourcing architecture.
// All state changes are persisted as immutable events.
type EventStore interface {
	// Append adds events to a stream (thread). Events are immutable once appended.
	// Returns the new version number after appending.
	Append(ctx context.Context, streamID string, events ...Event) (int64, error)

	// ReadStream reads all events for a stream from a specific version.
	// Use version 0 to read from the beginning.
	ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]Event, error)
}
