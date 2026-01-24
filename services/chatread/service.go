package chatread

import (
	"context"

	"school-with-ai/domain"
)

// ThreadStore provides read access to thread state.
type ThreadStore interface {
	Get(threadID string) (*domain.Thread, bool)
	All() []domain.Thread
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
func (s *ChatReadService) GetThread(ctx context.Context, threadID string) (*domain.Thread, bool) {
	return s.threadStore.Get(threadID)
}

// ListThreads retrieves all threads.
func (s *ChatReadService) ListThreads(ctx context.Context) []domain.Thread {
	return s.threadStore.All()
}
