package domain

import (
	"context"
	"log/slog"
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

	slog.Debug("Loading thread from event store",
		"stream", streamName,
		"eventCount", len(events))

	// Rebuild state by replaying events
	err = thread.LoadFromHistory(events, thread.Apply)
	if err != nil {
		return nil, err
	}

	slog.Debug("Thread loaded from history",
		"version", thread.Version,
		"title", thread.Title,
		"messageCount", len(thread.Messages))

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
	// thread.Version = number of persisted events (incremented only in LoadFromHistory)
	// expectedVersion = current stream version (number of events already in store)
	// Following EventSourcing.NetCore pattern: Version tracks persisted events, NOT in-memory changes
	expectedVersion := thread.Version

	slog.Debug("Saving thread aggregate",
		"stream", streamName,
		"version", thread.Version,
		"uncommittedEvents", len(uncommittedEvents),
		"expectedVersion", expectedVersion)

	_, err := r.eventStore.AppendWithVersion(ctx, streamName, expectedVersion, uncommittedEvents...)

	if err != nil {
		slog.Debug("Thread save failed", "stream", streamName, "error", err)
	} else {
		slog.Debug("Thread save succeeded", "stream", streamName)
	}

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

	slog.Debug("Saving new thread aggregate",
		"stream", streamName,
		"version", thread.Version,
		"uncommittedEvents", len(uncommittedEvents))

	// Use -2 to indicate NoStream (stream must not exist)
	// This is different from -1 (Any) and >= 0 (specific revision)
	_, err := r.eventStore.AppendWithVersion(ctx, streamName, -2, uncommittedEvents...)

	if err != nil {
		slog.Debug("Thread SaveNew failed", "stream", streamName, "error", err)
	} else {
		slog.Debug("Thread SaveNew succeeded", "stream", streamName)
	}

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

	// Use the decider or call aggregate method directly
	err = thread.Delete()
	if err != nil {
		return err
	}

	return r.Save(ctx, thread)
}
