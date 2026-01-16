package core

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// all conversation state in local memory.
//
// InMemoryThread is designed for scenarios where conversation state should be stored locally
// rather than in external services or databases. This approach provides high performance and
// simplicity while maintaining full control over the conversation data.
//
// In-memory threads do not persist conversation data across application restarts
// unless explicitly serialized and restored.
type InMemoryThread struct {
	id           string
	messageStore MessageStore
}

// NewInMemoryThread creates a new in-memory thread with a generated ID.
func NewInMemoryThread() *InMemoryThread {
	return &InMemoryThread{
		id:           uuid.New().String(),
		messageStore: NewInMemoryMessageStore(),
	}
}

// NewInMemoryThreadWithID creates a new in-memory thread with a specific ID.
func NewInMemoryThreadWithID(id string) *InMemoryThread {
	return &InMemoryThread{
		id:           id,
		messageStore: NewInMemoryMessageStore(),
	}
}

// NewInMemoryThreadWithMessages creates a new in-memory thread with initial messages.
func NewInMemoryThreadWithMessages(messages []Message) (*InMemoryThread, error) {
	thread := &InMemoryThread{
		id:           uuid.New().String(),
		messageStore: NewInMemoryMessageStore(),
	}

	err := thread.messageStore.AddMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to add initial messages: %w", err)
	}

	return thread, nil
}

// GetID returns the unique identifier for this thread.
func (t *InMemoryThread) GetID() string {
	return t.id
}

// RetrieveFullConversationHistory returns all messages in the thread.
func (t *InMemoryThread) RetrieveFullConversationHistory(ctx context.Context) ([]Message, error) {
	return t.messageStore.GetMessages(ctx)
}

// AddMessages adds new messages to the thread.
func (t *InMemoryThread) AddMessages(messages []Message) error {
	err := t.messageStore.AddMessages(messages)
	if err != nil {
		return fmt.Errorf("failed to add messages to thread: %w", err)
	}

	return nil
}

// GetMessageStore returns the underlying message store for advanced operations.
func (t *InMemoryThread) GetMessageStore() MessageStore {
	return t.messageStore
}
