package core

import (
	"context"
	"fmt"
)

// ThreadRepository manages thread state and events.
type ThreadRepository struct {
	eventStore EventStore
}

// NewThreadRepository creates a new thread repository.
func NewThreadRepository(eventStore EventStore) *ThreadRepository {
	return &ThreadRepository{
		eventStore: eventStore,
	}
}

// GetThreadEvents retrieves all events for a thread.
func (r *ThreadRepository) GetThreadEvents(ctx context.Context, threadID string) ([]Event, error) {
	events, err := r.eventStore.ReadStream(ctx, threadID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to read thread events: %w", err)
	}

	return events, nil
}

// AppendEvent appends an event to the thread.
func (r *ThreadRepository) AppendEvent(ctx context.Context, threadID string, event Event) error {
	_, err := r.eventStore.AppendWithVersion(ctx, threadID, -1, event)
	if err != nil {
		return fmt.Errorf("failed to append event: %w", err)
	}

	return nil
}
