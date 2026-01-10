package domain

import (
	"context"

	"school-with-ai/sdk/agentsdk/core"
)

// Repository provides a higher-level interface for working with aggregates.
// It handles loading from events, saving uncommitted events, and optimistic concurrency.

// ThreadRepository manages persistence of ThreadAggregate.
type ThreadRepository struct {
	eventStore core.EventStore
}

// NewThreadRepository creates a new thread repository.
func NewThreadRepository(eventStore core.EventStore) *ThreadRepository {
	return &ThreadRepository{
		eventStore: eventStore,
	}
}

// Load retrieves a thread aggregate by ID, rebuilding it from events.
func (r *ThreadRepository) Load(ctx context.Context, threadID string) (*ThreadAggregate, error) {
	thread := NewThreadAggregate(threadID)
	streamName := thread.StreamName()

	// Read all events for this thread
	events, err := r.eventStore.ReadStream(ctx, streamName, 0)
	if err != nil {
		return nil, err
	}

	// Rebuild state by replaying events
	err = thread.LoadFromHistory(events, func(e core.Event) {
		thread.Evolve(e)
	})
	if err != nil {
		return nil, err
	}

	return thread, nil
}

// Save persists any uncommitted events from the aggregate.
// Uses optimistic concurrency - will fail if stream version has changed.
func (r *ThreadRepository) Save(ctx context.Context, thread *ThreadAggregate) error {
	uncommittedEvents := thread.DequeueUncommittedEvents()
	if len(uncommittedEvents) == 0 {
		return nil // Nothing to save
	}

	streamName := thread.StreamName()

	// Save with optimistic concurrency check
	// Calculate expected version: current version minus number of uncommitted events
	// This gives us the stream version BEFORE we appended our new events
	expectedVersion := thread.Version - int64(len(uncommittedEvents))
	_, err := r.eventStore.AppendWithVersion(ctx, streamName, expectedVersion, uncommittedEvents...)

	return err
}

// SaveNew persists a new aggregate (expecting an empty stream).
// Uses expectedVersion = -2 as a special sentinel value to indicate NoStream state.
func (r *ThreadRepository) SaveNew(ctx context.Context, thread *ThreadAggregate) error {
	uncommittedEvents := thread.DequeueUncommittedEvents()
	if len(uncommittedEvents) == 0 {
		return nil
	}

	streamName := thread.StreamName()

	// Use -2 to indicate NoStream (stream must not exist)
	// This is different from -1 (Any) and >= 0 (specific revision)
	_, err := r.eventStore.AppendWithVersion(ctx, streamName, -2, uncommittedEvents...)

	return err
}

// Delete marks a thread as deleted by appending a ThreadDeleted event.
func (r *ThreadRepository) Delete(ctx context.Context, threadID string) error {
	thread, err := r.Load(ctx, threadID)
	if err != nil {
		return err
	}

	// Check if thread exists (has events)
	if thread.Version == 0 {
		return ErrThreadNotFound
	}

	// Use the decider to handle the delete command
	decider := NewThreadDecider()
	cmd := DeleteThread{ThreadID: threadID}

	events, err := decider.Decide(cmd, thread)
	if err != nil {
		return err
	}

	// Enqueue events and apply to in-memory state
	for _, event := range events {
		thread.Enqueue(event)
		thread.Evolve(event)
	}

	return r.Save(ctx, thread)
}
