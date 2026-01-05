package sdk

import (
	"context"

	"school-with-ai/domain"
)

// ThreadStore provides read access to thread state for agents.
// Agents depend on this interface to get conversation context.
type ThreadStore interface {
	// GetAgentThread retrieves the conversation state for agent processing.
	GetAgentThread(threadID string) (AgentThread, bool)

	// GetOrCreateAgentThread ensures a thread exists in the store.
	GetOrCreateAgentThread(ctx context.Context, threadID string) (AgentThread, error)

	// Get retrieves a thread by ID.
	Get(threadID string) (*domain.Thread, bool)

	// Messages retrieves all messages from a thread.
	Messages(threadID string) []domain.Message

	// All retrieves all threads sorted by most recent first.
	All() []domain.Thread

	// Update applies a single event to update the thread state.
	Update(event Event) error
}
