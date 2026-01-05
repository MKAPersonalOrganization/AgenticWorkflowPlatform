package sdk

import (
	"context"
	"errors"
)

// EventStore is the source of truth in the event sourcing architecture.
//
// All state changes are persisted as immutable events. The EventStore is
// append-only - events cannot be modified or deleted (except for GDPR compliance).
//
// Agents write to the EventStore by appending events.
// Agents read from Projections (which are built from EventStore events).
//
// Implementations:
//   - InMemoryEventStore: For development and testing
//   - PostgresEventStore: For production with PostgreSQL
//   - Other database backends as needed
type EventStore interface {
	// Append adds events to a stream (thread). Events are immutable once appended.
	// Returns the new version number after appending.
	Append(ctx context.Context, streamID string, events ...Event) (int64, error)

	// ReadStream reads all events for a stream from a specific version.
	// Use version 0 to read from the beginning.
	ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]Event, error)

	// ReadAll reads all events for a stream (convenience method for ReadStream(ctx, id, 0)).
	ReadAll(ctx context.Context, streamID string) ([]Event, error)

	// StreamVersion returns the current version (event count) for a stream.
	// Returns 0 if stream doesn't exist.
	StreamVersion(ctx context.Context, streamID string) (int64, error)

	// StreamExists checks if a stream has any events.
	StreamExists(ctx context.Context, streamID string) (bool, error)

	// ListStreams returns all stream IDs in the store.
	ListStreams(ctx context.Context) ([]string, error)

	// DeleteStream removes all events for a stream (for GDPR compliance).
	// This is the only destructive operation allowed.
	DeleteStream(ctx context.Context, streamID string) error
}

// EventStoreWithSubscription extends EventStore with real-time event subscription.
// This allows projections to be updated in real-time as events are appended.
type EventStoreWithSubscription interface {
	EventStore

	// Subscribe creates a subscription to events for a specific stream.
	// The handler is called for each new event appended to the stream.
	// Returns a function to unsubscribe.
	Subscribe(ctx context.Context, streamID string, handler EventHandler) (unsubscribe func(), err error)

	// SubscribeAll creates a subscription to all events across all streams.
	// Useful for building cross-stream projections.
	SubscribeAll(ctx context.Context, handler EventHandler) (unsubscribe func(), err error)
}

// EventHandler processes events from subscriptions.
type EventHandler func(ctx context.Context, streamID string, event Event) error

// Common errors for EventStore implementations.
var (
	// ErrStreamNotFound is returned when a stream doesn't exist.
	ErrStreamNotFound = errors.New("stream not found")

	// ErrConcurrencyConflict is returned when appending with an outdated expected version.
	ErrConcurrencyConflict = errors.New("concurrency conflict: stream was modified")
)

// EventStoreWithOptimisticLocking extends EventStore with optimistic concurrency control.
// This prevents lost updates when multiple writers try to append to the same stream.
type EventStoreWithOptimisticLocking interface {
	EventStore

	// AppendWithExpectedVersion appends events only if the stream is at the expected version.
	// This provides optimistic concurrency control.
	// Use expectedVersion = -1 to expect the stream doesn't exist (create new).
	// Returns ErrConcurrencyConflict if the expected version doesn't match.
	AppendWithExpectedVersion(ctx context.Context, streamID string, expectedVersion int64, events ...Event) (int64, error)
}
