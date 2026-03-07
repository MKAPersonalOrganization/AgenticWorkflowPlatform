package eventsourcing

import "context"

// EventStore interface defines the contract for persisting events.
// This interface allows the domain to be independent of the specific event store implementation.
type EventStore interface {
// AppendWithVersion adds events to a stream with optimistic concurrency control.
AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...Event) (int64, error)

// ReadStream reads all events for a stream from a specific version.
ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]Event, error)

// DeleteStream removes a stream and all its events.
DeleteStream(ctx context.Context, streamID string) error
}
