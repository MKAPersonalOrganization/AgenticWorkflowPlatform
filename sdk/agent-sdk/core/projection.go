package sdk

import (
	"context"
)

// Projection is a read model built from events.
// Projections are rebuilt by replaying events from the EventStore.
type Projection interface {
	// Apply updates the projection with a single event.
	Apply(ctx context.Context, event Event) error

	// Rebuild reconstructs the projection from the EventStore.
	Rebuild(ctx context.Context, store EventStore, threadID string) error

	// GetAgentThread retrieves the conversation state for agent processing.
	GetAgentThread(threadID string) (AgentThread, bool)
}
