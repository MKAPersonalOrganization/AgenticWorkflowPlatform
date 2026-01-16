package threadmgmt

import (
	"context"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// EventStoreAdapter adapts core.EventStore to domain.EventStore.
type EventStoreAdapter struct {
	store core.EventStore
}

func NewEventStoreAdapter(store core.EventStore) *EventStoreAdapter {
	return &EventStoreAdapter{store: store}
}

func (a *EventStoreAdapter) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...domain.Event) (int64, error) {
	coreEvents := make([]core.Event, len(events))
	for i, evt := range events {
		// Since domain.Event and core.Event have identical method signatures,
		// and the underlying concrete types implement those methods,
		// we can cast/assign them.
		coreEvents[i] = evt.(core.Event)
	}
	return a.store.AppendWithVersion(ctx, streamID, expectedVersion, coreEvents...)
}

func (a *EventStoreAdapter) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]domain.Event, error) {
	coreEvents, err := a.store.ReadStream(ctx, streamID, fromVersion)
	if err != nil {
		return nil, err
	}

	domainEvents := make([]domain.Event, len(coreEvents))
	for i, evt := range coreEvents {
		domainEvents[i] = evt.(domain.Event)
	}
	return domainEvents, nil
}

func (a *EventStoreAdapter) DeleteStream(ctx context.Context, streamID string) error {
	return a.store.DeleteStream(ctx, streamID)
}
