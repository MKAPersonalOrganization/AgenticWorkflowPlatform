package agent

import "school-with-ai/domain"

// ThreadState provides read access to the current state of threads
type ThreadState interface {
	// Get retrieves a thread by ID
	Get(threadID string) (*domain.Thread, bool)

	// Messages retrieves all messages from a thread
	Messages(threadID string) []domain.Message

	// All retrieves all threads sorted by most recent first
	All() []domain.Thread

	// Update applies a single event to update the state
	Update(event Event) error
}
