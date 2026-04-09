package chatread

import (
	"aicompass/modules/chat/thread"
	"context"
)

// ThreadStore provides read access to thread state.
type ThreadStore interface {
	Get(threadID string) (*thread.Thread, bool)
	All() []thread.Thread
}

// ChatReadService provides read operations for chat management.
type ChatReadService struct {
	threadStore ThreadStore
}

// New creates a new chat read service.
func New(threadStore ThreadStore) *ChatReadService {
	return &ChatReadService{
		threadStore: threadStore,
	}
}

// GetThread retrieves a thread by ID.
func (s *ChatReadService) GetThread(ctx context.Context, threadID string) (*thread.Thread, bool) {
	return s.threadStore.Get(threadID)
}

// ListThreads retrieves all threads.
func (s *ChatReadService) ListThreads(ctx context.Context) []thread.Thread {
	return s.threadStore.All()
}
