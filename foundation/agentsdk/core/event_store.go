package core

import (
	"context"
	"errors"
)

// ErrConcurrencyConflict is returned when optimistic concurrency check fails.
// This happens when expectedVersion doesn't match the actual stream version.
var ErrConcurrencyConflict = errors.New("concurrency conflict: stream version mismatch")

// EventStore is used internally by SDK for event persistence.
type EventStore interface {
	// AppendWithVersion adds events to a stream with optimistic concurrency control.
	// expectedVersion: the version the stream should be at before appending.
	// Use -1 to append to any version (no concurrency check).
	// Use 0 to ensure stream doesn't exist yet (first write).
	// Returns the new stream version after appending.
	AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...Event) (int64, error)

	// ReadStream reads all events for a stream from a specific version.
	ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]Event, error)

	// DeleteStream removes a stream and all its events.
	DeleteStream(ctx context.Context, streamID string) error
}
