package eventstore

import (
	"context"

	"school-with-ai/sdk/agentsdk/core"
)

// Projection is a read model built from events.
// Projections are rebuilt by replaying events from the EventStore.
type Projection interface {
	// Apply updates the projection with a single event.
	Apply(ctx context.Context, event core.Event) error

	// Rebuild reconstructs the projection from the EventStore.
	Rebuild(ctx context.Context, store core.EventStore, streamID string) error
}
