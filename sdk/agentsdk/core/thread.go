package core

import (
	"school-with-ai/domain"
)

// ThreadType represents the type of agent thread.
type ThreadType string

const (
	// ThreadTypeLocal represents a thread with all conversation state stored locally.
	ThreadTypeLocal ThreadType = "local"

	// ThreadTypeRemote represents a thread where conversation state is managed server-side.
	ThreadTypeRemote ThreadType = "remote"
)

// AgentThread is the base abstraction for all agent threads.
//
// An AgentThread contains the state of a specific conversation with an agent which may include:
//   - Conversation history or a reference to externally stored conversation history
//   - Memories or a reference to externally stored memories
//   - Any other state that the agent needs to persist across runs for a conversation
//
// An AgentThread may also have behaviors attached to it that may include:
//   - Customized storage of state
//   - Data extraction from and injection into a conversation
//   - Chat history reduction, e.g. where messages need to be summarized or truncated to reduce the size
//
// An AgentThread is always constructed by an Agent so that the Agent can attach any necessary
// behaviors to the AgentThread. See the Agent.GetNewThread() and Agent.DeserializeThread() methods.
//
// Because of these behaviors, an AgentThread may not be reusable across different agents, since each agent
// may add different behaviors to the AgentThread it creates.
//
// To support conversations that may need to survive application restarts or separate service requests,
// an AgentThread can be serialized and deserialized.
type AgentThread interface {
	// GetID returns the unique identifier for this thread
	GetID() string

	// GetMessages returns all messages in the thread
	GetMessages() []domain.Message

	// AddMessages adds new messages to the thread
	AddMessages(messages []domain.Message) error

	// Serialize serializes the thread state to JSON
	Serialize() ([]byte, error)

	// GetType returns the thread type (local or remote)
	GetType() ThreadType

	// GetService asks the AgentThread for an object of the specified type.
	// This allows for retrieval of strongly-typed services that might be provided by the thread,
	// including the thread itself or any services it might be wrapping.
	// Returns nil if the service is not available.
	GetService(serviceType any) any
}
