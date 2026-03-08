package thread

import (
	"context"

	es "school-with-ai/foundation/eventsourcing"
)

// ThreadRepository manages persistence of ThreadAggregate.
// It wraps the generic Repository[*ThreadAggregate] for type-specific operations.
type ThreadRepository struct {
	*Repository[*ThreadAggregate]
}

// NewThreadRepository creates a new thread repository using the generic repository pattern.
// If a SnapshotStore is provided, the repository will use snapshots to accelerate
// aggregate loading for threads with many events.
func NewThreadRepository(eventStore EventStore, snapshotStore ...es.SnapshotStore) *ThreadRepository {
	genericRepo := NewRepository(
		eventStore,
		NewThreadAggregate,
		func(t *ThreadAggregate) string { return t.StreamName() },
	)

	if len(snapshotStore) > 0 && snapshotStore[0] != nil {
		genericRepo = genericRepo.WithSnapshots(snapshotStore[0])
	}

	return &ThreadRepository{
		Repository: genericRepo,
	}
}

// Delete marks a thread as deleted by appending a ThreadDeleted event.
// This is a thread-specific operation that uses the generic repository's Load and Save.
func (r *ThreadRepository) Delete(ctx context.Context, threadID string) error {
	t, err := r.Load(ctx, threadID)
	if err != nil {
		return err
	}

	// Call the aggregate's Delete method
	err = t.Delete()
	if err != nil {
		return err
	}

	return r.Save(ctx, t)
}
