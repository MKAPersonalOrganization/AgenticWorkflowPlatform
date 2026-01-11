package domain

import (
	"context"

	"school-with-ai/sdk/agentsdk/core"
)

// Repository provides a generic interface for working with any aggregate that implements the Aggregate interface.
// It handles loading from events, saving uncommitted events, and optimistic concurrency.
// Following the EventSourcing.NetCore pattern for generic repository implementation.
//
// The aggregate type T must:
// - Implement the Aggregate interface
// - Have a LoadFromHistory method (typically via BaseAggregate embedding)
type Repository[T interface {
	Aggregate
	LoadFromHistory(events []core.Event, apply func(core.Event)) error
}] struct {
	eventStore    core.EventStore
	newAggregate  func(id string) T
	getStreamName func(T) string
}

// NewRepository creates a new generic repository for any aggregate type.
// Parameters:
//   - eventStore: The event store implementation
//   - newAggregate: Factory function to create a new aggregate instance
//   - getStreamName: Function to get the stream name from an aggregate
func NewRepository[T interface {
	Aggregate
	LoadFromHistory(events []core.Event, apply func(core.Event)) error
}](
	eventStore core.EventStore,
	newAggregate func(id string) T,
	getStreamName func(T) string,
) *Repository[T] {
	return &Repository[T]{
		eventStore:    eventStore,
		newAggregate:  newAggregate,
		getStreamName: getStreamName,
	}
}

// Load retrieves an aggregate by ID, rebuilding it from events.
// Returns ErrThreadNotFound if the stream doesn't exist (no events).
func (r *Repository[T]) Load(ctx context.Context, id string) (T, error) {
	aggregate := r.newAggregate(id)
	streamName := r.getStreamName(aggregate)

	// Read all events for this aggregate
	events, err := r.eventStore.ReadStream(ctx, streamName, 0)
	if err != nil {
		var zero T
		return zero, err
	}

	// Return error if aggregate doesn't exist (no events found)
	if len(events) == 0 {
		var zero T
		return zero, ErrThreadNotFound
	}

	// Rebuild state by replaying events
	// Call the aggregate's Apply method through LoadFromHistory
	err = aggregate.LoadFromHistory(events, aggregate.Apply)
	if err != nil {
		var zero T
		return zero, err
	}

	return aggregate, nil
}

// Save persists any uncommitted events from the aggregate.
// Uses optimistic concurrency - will fail if stream version has changed.
func (r *Repository[T]) Save(ctx context.Context, aggregate T) error {
	uncommittedEvents := aggregate.DequeueUncommittedEvents()
	if len(uncommittedEvents) == 0 {
		return nil // Nothing to save
	}

	streamName := r.getStreamName(aggregate)

	// Save with optimistic concurrency check
	// aggregate.Version = number of persisted events (incremented only in LoadFromHistory)
	// expectedVersion = current stream version (number of events already in store)
	// Following EventSourcing.NetCore pattern: Version tracks persisted events, NOT in-memory changes
	expectedVersion := aggregate.GetVersion()

	_, err := r.eventStore.AppendWithVersion(ctx, streamName, expectedVersion, uncommittedEvents...)

	return err
}

// SaveNew persists a new aggregate (expecting an empty stream).
// Uses expectedVersion = -2 as a special sentinel value to indicate NoStream state.
func (r *Repository[T]) SaveNew(ctx context.Context, aggregate T) error {
	uncommittedEvents := aggregate.DequeueUncommittedEvents()
	if len(uncommittedEvents) == 0 {
		return nil
	}

	streamName := r.getStreamName(aggregate)

	// Use -2 to indicate NoStream (stream must not exist)
	// This is different from -1 (Any) and >= 0 (specific revision)
	_, err := r.eventStore.AppendWithVersion(ctx, streamName, -2, uncommittedEvents...)

	return err
}
