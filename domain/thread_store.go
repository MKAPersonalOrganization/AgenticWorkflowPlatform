package domain

import (
	"context"
	"school-with-ai/foundation/agentsdk/core"
)

// ThreadStore provides read access to thread state from projections.
// This interface is implemented by projection read models (e.g., PostgresLLMContextProjection)
// and used by services that need to query thread state.
type ThreadStore interface {
	// GetAgentThread retrieves a thread as AgentThread for LLM processing.
	// Returns false if thread doesn't exist or is deleted.
	GetAgentThread(threadID string) (core.AgentThread, bool)

	// GetOrCreateAgentThread ensures a thread exists, creating an empty one if needed.
	GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error)

	// Get retrieves the full thread with metadata.
	Get(threadID string) (*Thread, bool)

	// Messages retrieves all messages from a thread.
	Messages(threadID string) []Message

	// All retrieves all non-deleted threads.
	All() []Thread
}
