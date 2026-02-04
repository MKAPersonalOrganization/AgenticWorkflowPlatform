package agent

import (
	"context"
	"errors"
)

// ErrStateKeyNotFound is returned when a state key doesn't exist.
var ErrStateKeyNotFound = errors.New("state key does not exist")

// Session manages conversation state and history.
// This interface is compatible with both ADK-Go and Blades session concepts.
type Session interface {
	// ID returns the session identifier.
	ID() string

	// UserID returns the user identifier.
	UserID() string

	// AppName returns the application name (optional).
	AppName() string

	// State returns the session state storage.
	State() State

	// History returns all messages in the conversation.
	History() []*Message

	// Append adds a new message to the conversation.
	Append(context.Context, *Message) error

	// LastUpdateTime returns when the session was last modified.
	LastUpdateTime() int64
}

// State provides key-value storage for session data.
// This is useful for sharing state between agents and tools.
//
// State keys can have prefixes for scoping (ADK-Go pattern):
// - "app:" - Shared across all users and sessions
// - "user:" - Shared across all sessions for a user
// - "temp:" - Specific to current invocation (not persisted)
// - No prefix - Session-scoped (default)
type State interface {
	// Get retrieves a value by key.
	// Returns ErrStateKeyNotFound if the key doesn't exist.
	Get(key string) (any, error)

	// Set stores a value by key.
	Set(key string, value any) error

	// Delete removes a value by key.
	Delete(key string) error

	// Keys returns all stored keys.
	Keys() []string

	// All returns an iterator over all key-value pairs.
	All() func(yield func(key string, value any) bool)
}

// SessionStore manages session persistence.
type SessionStore interface {
	// Get retrieves a session by ID.
	Get(ctx context.Context, appName, userID, sessionID string) (Session, error)

	// Create initializes a new session.
	Create(ctx context.Context, appName, userID string) (Session, error)

	// Save persists session changes.
	Save(ctx context.Context, session Session) error

	// Delete removes a session.
	Delete(ctx context.Context, appName, userID, sessionID string) error

	// List returns all sessions for a user.
	List(ctx context.Context, appName, userID string) ([]Session, error)
}

// MutableSession extends Session with write operations.
// Used when the session needs to be modified during agent execution.
type MutableSession interface {
	Session

	// SetState updates the session state.
	SetState(key string, value any) error

	// AppendEvent adds an event to the session history.
	// This is used by ADK-Go style frameworks.
	AppendEvent(ctx context.Context, event Event) error
}

// InMemorySession provides a simple in-memory Session implementation.
// Useful for testing and single-request scenarios.
type InMemorySession struct {
	id         string
	userID     string
	appName    string
	state      map[string]any
	history    []*Message
	lastUpdate int64
}

// NewInMemorySession creates a new in-memory session.
func NewInMemorySession(id, userID, appName string) *InMemorySession {
	return &InMemorySession{
		id:      id,
		userID:  userID,
		appName: appName,
		state:   make(map[string]any),
		history: make([]*Message, 0),
	}
}

func (s *InMemorySession) ID() string            { return s.id }
func (s *InMemorySession) UserID() string        { return s.userID }
func (s *InMemorySession) AppName() string       { return s.appName }
func (s *InMemorySession) LastUpdateTime() int64 { return s.lastUpdate }

func (s *InMemorySession) State() State {
	return &inMemoryState{data: s.state}
}

func (s *InMemorySession) History() []*Message {
	return s.history
}

func (s *InMemorySession) Append(ctx context.Context, msg *Message) error {
	s.history = append(s.history, msg)
	s.lastUpdate = msg.Timestamp.Unix()
	return nil
}

type inMemoryState struct {
	data map[string]any
}

func (s *inMemoryState) Get(key string) (any, error) {
	v, ok := s.data[key]
	if !ok {
		return nil, ErrStateKeyNotFound
	}
	return v, nil
}

func (s *inMemoryState) Set(key string, value any) error {
	s.data[key] = value
	return nil
}

func (s *inMemoryState) Delete(key string) error {
	delete(s.data, key)
	return nil
}

func (s *inMemoryState) Keys() []string {
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

func (s *inMemoryState) All() func(yield func(key string, value any) bool) {
	return func(yield func(key string, value any) bool) {
		for k, v := range s.data {
			if !yield(k, v) {
				return
			}
		}
	}
}
