package thread

import (
	"context"
)

// ThreadRepository manages persistence of ThreadAggregate.
// It wraps the generic Repository[*ThreadAggregate] for type-specific operations.
type ThreadRepository struct {
	*Repository[*ThreadAggregate]
}

// NewThreadRepository creates a new thread repository using the generic repository pattern.
func NewThreadRepository(eventStore EventStore) *ThreadRepository {
	genericRepo := NewRepository(
		eventStore,
		NewThreadAggregate,
		func(t *ThreadAggregate) string { return t.StreamName() },
	)

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
