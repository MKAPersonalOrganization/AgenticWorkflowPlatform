package chatread

import (
	"context"

	"school-with-ai/domain"
	sdk "school-with-ai/sdk/agent-sdk/core"
)

// ChatReadService provides read operations for chat management
type ChatReadService struct {
	threadStore sdk.ThreadStore
}

// New creates a new chat read service
func New(threadStore sdk.ThreadStore) *ChatReadService {
	return &ChatReadService{
		threadStore: threadStore,
	}
}

// GetThread retrieves a thread by ID
func (s *ChatReadService) GetThread(ctx context.Context, threadID string) (*domain.Thread, bool) {
	return s.threadStore.Get(threadID)
}

// ListThreads retrieves all threads
func (s *ChatReadService) ListThreads(ctx context.Context) []domain.Thread {
	return s.threadStore.All()
}
