package custom

import (
	"context"
	"school-with-ai/modules/chat/thread"
	"school-with-ai/foundation/agentsdk/core"
)

// CoreEventStoreAdapter adapts thread.EventStore to core.EventStore.
// This allows the SDK to use the domain-level event store without knowing about it.
type CoreEventStoreAdapter struct {
	store thread.EventStore
}

func NewCoreEventStoreAdapter(store thread.EventStore) *CoreEventStoreAdapter {
	return &CoreEventStoreAdapter{store: store}
}

func (a *CoreEventStoreAdapter) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...core.Event) (int64, error) {
	domainEvents := make([]thread.Event, len(events))
	for i, evt := range events {
		domainEvents[i] = evt.(thread.Event)
	}
	return a.store.AppendWithVersion(ctx, streamID, expectedVersion, domainEvents...)
}

func (a *CoreEventStoreAdapter) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
	domainEvents, err := a.store.ReadStream(ctx, streamID, fromVersion)
	if err != nil {
		return nil, err
	}

	coreEvents := make([]core.Event, len(domainEvents))
	for i, evt := range domainEvents {
		coreEvents[i] = evt.(core.Event)
	}
	return coreEvents, nil
}

func (a *CoreEventStoreAdapter) DeleteStream(ctx context.Context, streamID string) error {
	return a.store.DeleteStream(ctx, streamID)
}
