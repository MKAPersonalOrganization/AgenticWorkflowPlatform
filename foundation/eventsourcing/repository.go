package eventsourcing

import (
	"context"
	"errors"
)

// ErrNotFound is returned when an aggregate has no events in the store.
var ErrNotFound = errors.New("aggregate not found")

// ErrConcurrencyConflict is returned when a stream version mismatch is detected
// during optimistic concurrency checks.
var ErrConcurrencyConflict = errors.New("concurrency conflict: stream version mismatch")

// Repository provides a generic interface for working with any aggregate that
// implements the Aggregate interface. It handles loading from events, saving
// uncommitted events, and optimistic concurrency.
//
// When a SnapshotStore is configured and the aggregate implements Snapshotable,
// Load will restore from the latest snapshot and replay only subsequent events.
// Save will periodically persist a new snapshot after SnapshotInterval events.
type Repository[T interface {
	Aggregate
	LoadFromHistory(events []Event, apply func(Event)) error
}] struct {
	eventStore    EventStore
	snapshotStore SnapshotStore
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

// WithSnapshots returns a copy of the repository configured to use the given
// SnapshotStore. The aggregate type must implement Snapshotable for this to
// take effect — otherwise snapshots are silently skipped.
func (r *Repository[T]) WithSnapshots(store SnapshotStore) *Repository[T] {
	return &Repository[T]{
		eventStore:    r.eventStore,
		snapshotStore: store,
		newAggregate:  r.newAggregate,
		getStreamName: r.getStreamName,
	}
}

// Load retrieves an aggregate by ID, rebuilding it from events.
// If a SnapshotStore is configured and the aggregate is Snapshotable,
// it loads the latest snapshot first and replays only newer events.
// Returns ErrNotFound if the stream doesn't exist (no events).
func (r *Repository[T]) Load(ctx context.Context, id string) (T, error) {
	aggregate := r.newAggregate(id)
	streamName := r.getStreamName(aggregate)

	var fromVersion int64

	// Try loading from snapshot if available.
	if r.snapshotStore != nil {
		if snapshotable, ok := any(aggregate).(Snapshotable); ok {
			snap, err := r.snapshotStore.LoadSnapshot(ctx, streamName)
			if err == nil && snap != nil {
				if err := snapshotable.UnmarshalSnapshot(snap.Data); err == nil {
					// Restore version from snapshot.
					if ba, ok := any(aggregate).(interface{ SetVersion(int64) }); ok {
						ba.SetVersion(snap.Version)
					}
					fromVersion = snap.Version
				}
				// If unmarshal fails, fall through to full replay.
			}
		}
	}

	events, err := r.eventStore.ReadStream(ctx, streamName, fromVersion)
	if err != nil {
		var zero T
		return zero, err
	}

	// If no snapshot and no events → not found.
	if fromVersion == 0 && len(events) == 0 {
		var zero T
		return zero, ErrNotFound
	}

	if len(events) > 0 {
		err = aggregate.LoadFromHistory(events, aggregate.Apply)
		if err != nil {
			var zero T
			return zero, err
		}
	}

	return aggregate, nil
}

// Save persists any uncommitted events from the aggregate.
// Uses optimistic concurrency — will fail if stream version has changed.
// Periodically takes a snapshot if the aggregate is Snapshotable.
func (r *Repository[T]) Save(ctx context.Context, aggregate T) error {
	uncommittedEvents := aggregate.DequeueUncommittedEvents()
	if len(uncommittedEvents) == 0 {
		return nil
	}

	streamName := r.getStreamName(aggregate)
	expectedVersion := aggregate.GetVersion()

	_, err := r.eventStore.AppendWithVersion(ctx, streamName, expectedVersion, uncommittedEvents...)
	if err != nil {
		return err
	}

	// Take a snapshot if we've crossed a snapshot interval boundary.
	if r.snapshotStore != nil {
		newVersion := expectedVersion + int64(len(uncommittedEvents))
		if snapshotable, ok := any(aggregate).(Snapshotable); ok {
			if newVersion >= SnapshotInterval && newVersion/SnapshotInterval > expectedVersion/SnapshotInterval {
				data, marshalErr := snapshotable.MarshalSnapshot()
				if marshalErr == nil {
					// Best-effort — snapshot failures don't break the write path.
					_ = r.snapshotStore.SaveSnapshot(ctx, &Snapshot{
						StreamName: streamName,
						Version:    newVersion,
						Data:       data,
					})
				}
			}
		}
	}

	return nil
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
