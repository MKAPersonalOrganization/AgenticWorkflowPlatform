package services

import (
	"context"
	"time"

	"school-with-ai/domain"
	sdk "school-with-ai/sdk/agent-sdk/core"
)

// ChatService provides high-level operations for chat management
// It encapsulates event emission and state management
type ChatService struct {
	eventStore  sdk.EventStore
	threadStore sdk.ThreadStore
}

// NewChatService creates a new chat service
func NewChatService(eventStore sdk.EventStore, threadStore sdk.ThreadStore) *ChatService {
	return &ChatService{
		eventStore:  eventStore,
		threadStore: threadStore,
	}
}

// CreateThread creates a new chat thread
func (s *ChatService) CreateThread(ctx context.Context, threadID, title string) error {
	// Check if thread already exists
	if _, exists := s.threadStore.Get(threadID); exists {
		return nil // Thread already exists, no-op
	}

	event := &sdk.ThreadCreatedEvent{
		BaseEvent: sdk.BaseEvent{
			Type:      "ThreadCreated",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		Title: title,
	}

	_, err := s.eventStore.Append(ctx, threadID, event)
	return err
}

// AddUserMessage adds a user message to a thread
func (s *ChatService) AddUserMessage(ctx context.Context, threadID, messageID, content string) error {
	event := &sdk.UserMessageAddedEvent{
		BaseEvent: sdk.BaseEvent{
			Type:      "UserMessageAdded",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
		Content:   content,
	}

	_, err := s.eventStore.Append(ctx, threadID, event)
	return err
}

// UpdateThreadTitle updates the title of a thread
func (s *ChatService) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) error {
	event := &sdk.ThreadTitleUpdatedEvent{
		BaseEvent: sdk.BaseEvent{
			Type:      "ThreadTitleUpdated",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		NewTitle: newTitle,
	}

	_, err := s.eventStore.Append(ctx, threadID, event)
	return err
}

// DeleteThread deletes a thread
func (s *ChatService) DeleteThread(ctx context.Context, threadID string) error {
	// Check if thread exists
	if _, exists := s.threadStore.Get(threadID); !exists {
		return domain.ErrThreadNotFound
	}

	event := &sdk.ThreadDeletedEvent{
		BaseEvent: sdk.BaseEvent{
			Type:      "ThreadDeleted",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
	}

	_, err := s.eventStore.Append(ctx, threadID, event)
	return err
}

// GetThread retrieves a thread by ID
func (s *ChatService) GetThread(ctx context.Context, threadID string) (*domain.Thread, bool) {
	return s.threadStore.Get(threadID)
}

// ListThreads retrieves all threads
func (s *ChatService) ListThreads(ctx context.Context) []domain.Thread {
	return s.threadStore.All()
}
