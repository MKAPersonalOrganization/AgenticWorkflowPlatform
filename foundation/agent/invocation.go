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

	// Artifacts provides access to artifact storage (optional).
	// Returns nil if artifacts are not supported.
	Artifacts() Artifacts

	// UserContent returns the user's original input for this invocation.
	UserContent() *Message

	// RunConfig returns the runtime configuration for this invocation.
	RunConfig() *RunConfig

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
