package eventsourcing

import (
"context"
"errors"
)

// ErrNotFound is returned when an aggregate has no events in the store.
var ErrNotFound = errors.New("aggregate not found")

// Repository provides a generic interface for working with any aggregate that
// implements the Aggregate interface. It handles loading from events, saving
// uncommitted events, and optimistic concurrency.
//
// The aggregate type T must:
//   - Implement the Aggregate interface
//   - Have a LoadFromHistory method (typically via BaseAggregate embedding)
type Repository[T interface {
Aggregate
LoadFromHistory(events []Event, apply func(Event)) error
}] struct {
eventStore    EventStore
newAggregate  func(id string) T
getStreamName func(T) string
}

// NewRepository creates a new generic repository for any aggregate type.
func NewRepository[T interface {
Aggregate
LoadFromHistory(events []Event, apply func(Event)) error
}](
eventStore EventStore,
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
// Returns ErrNotFound if the stream doesn't exist (no events).
func (r *Repository[T]) Load(ctx context.Context, id string) (T, error) {
aggregate := r.newAggregate(id)
streamName := r.getStreamName(aggregate)

events, err := r.eventStore.ReadStream(ctx, streamName, 0)
if err != nil {
var zero T
return zero, err
}

if len(events) == 0 {
var zero T
return zero, ErrNotFound
}

err = aggregate.LoadFromHistory(events, aggregate.Apply)
if err != nil {
var zero T
return zero, err
}

return aggregate, nil
}

// Save persists any uncommitted events from the aggregate.
// Uses optimistic concurrency — will fail if stream version has changed.
func (r *Repository[T]) Save(ctx context.Context, aggregate T) error {
uncommittedEvents := aggregate.DequeueUncommittedEvents()
if len(uncommittedEvents) == 0 {
return nil
}

streamName := r.getStreamName(aggregate)
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
_, err := r.eventStore.AppendWithVersion(ctx, streamName, -2, uncommittedEvents...)
return err
}
