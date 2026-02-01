package agent

import "context"

// Session manages conversation state and history.
// This interface is compatible with both ADK-Go and Blades session concepts.
type Session interface {
	// ID returns the session identifier.
	ID() string

	// UserID returns the user identifier.
	UserID() string

	// State returns the session state storage.
	State() State

	// History returns all messages in the conversation.
	History() []*Message

	// Append adds a new message to the conversation.
	Append(context.Context, *Message) error

	// SetTitle updates the conversation title.
	SetTitle(context.Context, string) error
}

// State provides key-value storage for session data.
// This is useful for sharing state between agents and tools.
type State interface {
	// Get retrieves a value by key.
	Get(key string) (any, bool)

	// Set stores a value by key.
	Set(key string, value any)

	// Delete removes a value by key.
	Delete(key string)

	// Keys returns all stored keys.
	Keys() []string
}

// SessionStore manages session persistence.
type SessionStore interface {
	// Get retrieves a session by ID.
	Get(ctx context.Context, sessionID string) (Session, error)

	// Create initializes a new session.
	Create(ctx context.Context, userID string) (Session, error)

	// Save persists session changes.
	Save(ctx context.Context, session Session) error

	// Delete removes a session.
	Delete(ctx context.Context, sessionID string) error
}
