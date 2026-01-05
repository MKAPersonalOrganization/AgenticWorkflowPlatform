package sdk

import (
	"encoding/json"
	"fmt"

	"school-with-ai/domain"

	"github.com/google/uuid"
)

// InMemoryThread provides an abstract base for agent threads that maintain
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

// inMemoryThreadState is used for JSON serialization/deserialization
type inMemoryThreadState struct {
	ID         string          `json:"id"`
	StoreState json.RawMessage `json:"storeState,omitempty"`
}

// NewInMemoryThread creates a new in-memory thread with a generated ID
func NewInMemoryThread() *InMemoryThread {
	return &InMemoryThread{
		id:           uuid.New().String(),
		messageStore: NewInMemoryMessageStore(),
	}
}

// NewInMemoryThreadWithID creates a new in-memory thread with a specific ID
func NewInMemoryThreadWithID(id string) *InMemoryThread {
	return &InMemoryThread{
		id:           id,
		messageStore: NewInMemoryMessageStore(),
	}
}

// NewInMemoryThreadWithMessages creates a new in-memory thread with initial messages
func NewInMemoryThreadWithMessages(messages []domain.Message) (*InMemoryThread, error) {
	thread := &InMemoryThread{
		id:           uuid.New().String(),
		messageStore: NewInMemoryMessageStore(),
	}

	if err := thread.messageStore.AddMessages(messages); err != nil {
		return nil, fmt.Errorf("failed to add initial messages: %w", err)
	}

	return thread, nil
}

// NewInMemoryThreadFromState creates a new in-memory thread from previously serialized state
func NewInMemoryThreadFromState(data []byte) (*InMemoryThread, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("serialized thread state is empty")
	}

	var state inMemoryThreadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to deserialize thread state: %w", err)
	}

	// Create message store from state
	messageStore, err := NewInMemoryMessageStoreFromState(state.StoreState)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize message store: %w", err)
	}

	return &InMemoryThread{
		id:           state.ID,
		messageStore: messageStore,
	}, nil
}

// GetID returns the unique identifier for this thread
func (t *InMemoryThread) GetID() string {
	return t.id
}

// GetMessages returns all messages in the thread
func (t *InMemoryThread) GetMessages() []domain.Message {
	return t.messageStore.GetMessages()
}

// AddMessages adds new messages to the thread
func (t *InMemoryThread) AddMessages(messages []domain.Message) error {
	return t.messageStore.AddMessages(messages)
}

// Serialize serializes the thread state to JSON
func (t *InMemoryThread) Serialize() ([]byte, error) {
	storeData, err := t.messageStore.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message store: %w", err)
	}

	state := inMemoryThreadState{
		ID:         t.id,
		StoreState: storeData,
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize thread state: %w", err)
	}

	return data, nil
}

// GetType returns the thread type (always local for InMemoryThread)
func (t *InMemoryThread) GetType() ThreadType {
	return ThreadTypeLocal
}

// GetService asks the thread for an object of the specified type
func (t *InMemoryThread) GetService(serviceType interface{}) interface{} {
	// Return self if requesting AgentThread type
	if _, ok := serviceType.(*AgentThread); ok {
		return t
	}

	// Return message store if requesting MessageStore type
	if _, ok := serviceType.(*MessageStore); ok {
		return t.messageStore
	}

	return nil
}

// GetMessageStore returns the underlying message store for advanced operations
func (t *InMemoryThread) GetMessageStore() MessageStore {
	return t.messageStore
}
