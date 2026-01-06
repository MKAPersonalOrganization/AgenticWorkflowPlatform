package core

import (
	"context"

	"school-with-ai/domain"
)

// ThreadState is a projection that provides read access to thread state.
//
// IMPORTANT: This is a READ MODEL built from events, not a primary storage.
// In event sourcing:
//   - EventStore is the source of truth (append-only events)
//   - ThreadState is a projection (derived from events)
//   - Agents READ from ThreadState
//   - Agents WRITE by emitting events to EventStore
//
// ThreadState is updated by applying events via the Update() method.
// It can be rebuilt at any time by replaying events from the EventStore.
type ThreadState interface {
	// Get retrieves a thread projection by ID.
	// Returns the derived thread state from events.
	Get(threadID string) (*domain.Thread, bool)

	// Messages retrieves all messages from a thread's projection.
	Messages(threadID string) []domain.Message

	// All retrieves all thread projections sorted by most recent first.
	All() []domain.Thread

	// Update applies a single event to update the projection.
	// This is called when new events are appended to the EventStore.
	Update(event Event) error
}

// ThreadStateWithAgentAccess extends ThreadState with methods that agents need
// to access thread conversation state for processing.
//
// This interface provides:
//   - GetAgentThread: Access to conversation state for LLM calls
//   - GetOrCreateAgentThread: Ensures thread exists in projection
//   - Rebuild: Reconstruct projection from EventStore
//
// REMEMBER: Agents read from this projection. Writes go to EventStore as events.
type ThreadStateWithAgentAccess interface {
	ThreadState

	// GetAgentThread retrieves the conversation state for agent processing.
	// This provides the messages/context the agent needs for LLM calls.
	GetAgentThread(threadID string) (AgentThread, bool)

	// GetOrCreateAgentThread ensures a thread exists in the projection.
	// If the thread doesn't exist, it creates an empty projection entry.
	// Note: This doesn't write to EventStore - it just ensures the projection
	// is ready to receive events for this thread.
	GetOrCreateAgentThread(ctx context.Context, threadID string) (AgentThread, error)

	// Rebuild reconstructs the projection from the EventStore.
	// This replays all events to rebuild the derived state.
	Rebuild(ctx context.Context, store EventStore, threadID string) error

	// RebuildAll reconstructs all thread projections from the EventStore.
	RebuildAll(ctx context.Context, store EventStore) error
}

// --- Backward Compatibility ---
// ThreadStateWithStore is kept for backward compatibility.
// Prefer using ThreadStateWithAgentAccess which better describes the role.
type ThreadStateWithStore = ThreadStateWithAgentAccess
