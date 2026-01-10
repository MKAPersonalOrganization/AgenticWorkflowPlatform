package core

import (
	"encoding/json"
	"fmt"
	"sync"
)

// MessageStore provides an interface for storing and retrieving chat messages.
// This abstraction allows for different storage backends and message reduction strategies.
type MessageStore interface {
	// AddMessages adds new messages to the store
	AddMessages(messages []Message) error

	// GetMessages returns all messages from the store
	GetMessages() []Message

	// GetMessagesByRole returns messages filtered by role
	GetMessagesByRole(role string) []Message

	// Clear removes all messages from the store
	Clear() error

	// Count returns the number of messages in the store
	Count() int

	// Serialize serializes the message store state to JSON
	Serialize() ([]byte, error)
}

// InMemoryMessageStore provides an in-memory implementation of MessageStore.
// It stores chat messages entirely in local memory, providing fast access and manipulation capabilities.
type InMemoryMessageStore struct {
	mu       sync.RWMutex
	messages []Message
}

// NewInMemoryMessageStore creates a new in-memory message store.
func NewInMemoryMessageStore() *InMemoryMessageStore {
	return &InMemoryMessageStore{
		messages: make([]Message, 0),
	}
}

// NewInMemoryMessageStoreFromState creates a new in-memory message store from serialized state.
func NewInMemoryMessageStoreFromState(data []byte) (*InMemoryMessageStore, error) {
	store := &InMemoryMessageStore{}

	if len(data) == 0 {
		store.messages = make([]Message, 0)

		return store, nil
	}

	var state struct {
		Messages []Message `json:"messages"`
	}

	err := json.Unmarshal(data, &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal message store state: %w", err)
	}

	store.messages = state.Messages
	if store.messages == nil {
		store.messages = make([]Message, 0)
	}

	return store, nil
}

// AddMessages adds new messages to the store.
func (s *InMemoryMessageStore) AddMessages(messages []Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = append(s.messages, messages...)

	return nil
}

// GetMessages returns all messages from the store.
func (s *InMemoryMessageStore) GetMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external mutation
	result := make([]Message, len(s.messages))
	copy(result, s.messages)

	return result
}

// GetMessagesByRole returns messages filtered by role.
func (s *InMemoryMessageStore) GetMessagesByRole(role string) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Message, 0)
	for _, msg := range s.messages {
		if msg.Role == role {
			result = append(result, msg)
		}
	}

	return result
}

// Clear removes all messages from the store.
func (s *InMemoryMessageStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = make([]Message, 0)

	return nil
}

// Count returns the number of messages in the store.
func (s *InMemoryMessageStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.messages)
}

// Serialize serializes the message store state to JSON.
func (s *InMemoryMessageStore) Serialize() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state := struct {
		Messages []Message `json:"messages"`
	}{
		Messages: s.messages,
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message store state: %w", err)
	}

	return data, nil
}
