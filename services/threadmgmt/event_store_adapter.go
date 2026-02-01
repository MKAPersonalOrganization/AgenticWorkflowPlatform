package threadmgmt

import (
	"context"

	"school-with-ai/domain"
)

// EventStoreAdapter adapts domain.EventStore.
type EventStoreAdapter struct {
	store domain.EventStore
}

func NewEventStoreAdapter(store domain.EventStore) *EventStoreAdapter {
	return &EventStoreAdapter{store: store}
}

func (a *EventStoreAdapter) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...domain.Event) (int64, error) {
	return a.store.AppendWithVersion(ctx, streamID, expectedVersion, events...)
}

func (a *EventStoreAdapter) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]domain.Event, error) {
	return a.store.ReadStream(ctx, streamID, fromVersion)
}

func (a *EventStoreAdapter) DeleteStream(ctx context.Context, streamID string) error {
	return a.store.DeleteStream(ctx, streamID)
}
