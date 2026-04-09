package threadmgmt

import (
	"aicompass/modules/chat/thread"
	"context"
)

// EventStoreAdapter adapts thread.EventStore.
type EventStoreAdapter struct {
	store thread.EventStore
}

func NewEventStoreAdapter(store thread.EventStore) *EventStoreAdapter {
	return &EventStoreAdapter{store: store}
}

func (a *EventStoreAdapter) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...thread.Event) (int64, error) {
	return a.store.AppendWithVersion(ctx, streamID, expectedVersion, events...)
}

func (a *EventStoreAdapter) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]thread.Event, error) {
	return a.store.ReadStream(ctx, streamID, fromVersion)
}

func (a *EventStoreAdapter) DeleteStream(ctx context.Context, streamID string) error {
	return a.store.DeleteStream(ctx, streamID)
}
