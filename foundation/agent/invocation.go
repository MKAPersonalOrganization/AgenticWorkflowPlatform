package agent

import (
	"context"
)

// InvocationContext provides rich context for agent execution.
// This interface extends context.Context with agent-specific information,
// similar to ADK-Go's InvocationContext.
//
// Implementations should be created by the adapter layer and passed to Agent.Run().
type InvocationContext interface {
	context.Context

	// InvocationID returns a unique identifier for this invocation.
	// An invocation starts with user input and ends with a final response.
	InvocationID() string

	// Agent returns the currently executing agent.
	Agent() Agent

	// Session provides access to conversation state and history.
	Session() Session

	// Artifacts provides access to artifact storage (optional).
	// Returns nil if artifacts are not supported.
	Artifacts() Artifacts

	// Memory provides access to long-term memory (optional).
	// Returns nil if memory is not supported.
	Memory() Memory

	// UserContent returns the user's original input for this invocation.
	UserContent() *Message

	// RunConfig returns the runtime configuration for this invocation.
	RunConfig() *RunConfig

	// Branch returns the execution branch for sub-agent isolation.
	// Format: "agent_1.agent_2.agent_3" where agent_1 is parent of agent_2.
	// Empty string for root agent.
	Branch() string

	// EndInvocation signals that the invocation should terminate.
	// Subsequent agent runs will be skipped.
	EndInvocation()

	// Ended returns true if EndInvocation() was called.
	Ended() bool
}

// ReadonlyContext provides read-only access to invocation data.
// Used for callbacks and tool predicates where state modification is not allowed.
type ReadonlyContext interface {
	context.Context

	// InvocationID returns the invocation identifier.
	InvocationID() string

	// AgentName returns the current agent's name.
	AgentName() string

	// UserID returns the user identifier.
	UserID() string

	// SessionID returns the session identifier.
	SessionID() string

	// Branch returns the execution branch.
	Branch() string

	// UserContent returns the user's original input.
	UserContent() *Message

	// State provides read-only access to session state.
	State() ReadonlyState
}

// ReadonlyState provides read-only access to session state.
type ReadonlyState interface {
	// Get retrieves a value by key.
	// Returns ErrStateKeyNotFound if the key doesn't exist.
	Get(key string) (any, error)

	// All returns an iterator over all key-value pairs.
	All() func(yield func(key string, value any) bool)
}

// Artifacts provides access to artifact storage for the session.
// Artifacts are user-uploaded or agent-generated files associated with a session.
type Artifacts interface {
	// Save stores an artifact with the given name.
	Save(ctx context.Context, name string, data []byte, mimeType string) error

	// Load retrieves an artifact by name.
	Load(ctx context.Context, name string) ([]byte, string, error)

	// List returns all artifact names.
	List(ctx context.Context) ([]string, error)
}

// Memory provides access to long-term memory across sessions.
// Memory enables agents to recall information from past interactions.
type Memory interface {
	// Search performs semantic search on stored memories.
	Search(ctx context.Context, query string, limit int) ([]MemoryResult, error)

	// Store saves a memory entry.
	Store(ctx context.Context, content string, metadata map[string]any) error
}

// MemoryResult represents a single memory search result.
type MemoryResult struct {
	// Content is the remembered information.
	Content string

	// Score indicates relevance (0.0-1.0).
	Score float64

	// Metadata contains additional context.
	Metadata map[string]any
}
