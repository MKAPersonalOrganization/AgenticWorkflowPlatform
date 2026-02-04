package custom

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// SessionAdapter adapts domain.ThreadAggregate to the agent.Session interface.
type SessionAdapter struct {
	aggregate *domain.ThreadAggregate
	state     *MapState
	mu        sync.RWMutex
}

// NewSessionAdapter creates a session adapter from a ThreadAggregate.
func NewSessionAdapter(aggregate *domain.ThreadAggregate) agent.Session {
	return &SessionAdapter{
		aggregate: aggregate,
		state:     NewMapState(),
	}
}

// ID returns the session identifier.
func (s *SessionAdapter) ID() string {
	return s.aggregate.GetID()
}

// UserID returns the user identifier.
func (s *SessionAdapter) UserID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aggregate.UserID
}

// State returns the session state storage.
func (s *SessionAdapter) State() agent.State {
	return s.state
}

// History returns all messages in the conversation.
func (s *SessionAdapter) History() []*agent.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := s.aggregate.Messages
	result := make([]*agent.Message, 0, len(messages))

	for _, msg := range messages {
		result = append(result, convertDomainMessageToAgent(msg))
	}

	return result
}

// Append adds a new message to the conversation.
func (s *SessionAdapter) Append(ctx context.Context, msg *agent.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert agent.Message to core.Message parts
	coreMsg := convertAgentMessageToCore(msg)

	// Add to aggregate
	return s.aggregate.AddMessage(
		coreMsg.ID,
		coreMsg.Role,
		coreMsg.Content,
		toDomainToolCalls(coreMsg.ToolCalls),
		coreMsg.ToolCallID,
		coreMsg.ToolName,
		coreMsg.AgentName,
		coreMsg.Metadata,
	)
}

func toDomainToolCalls(ctcs []core.ToolCall) []domain.ToolCall {
	if ctcs == nil {
		return nil
	}
	result := make([]domain.ToolCall, len(ctcs))
	for i, ctc := range ctcs {
		result[i] = domain.ToolCall{
			ID:        ctc.ID,
			Type:      ctc.Type,
			Name:      ctc.Name,
			Arguments: ctc.Arguments,
		}
	}
	return result
}

// AppName returns the application name.
func (s *SessionAdapter) AppName() string {
	return "schoolai"
}

// LastUpdateTime returns when the session was last modified.
func (s *SessionAdapter) LastUpdateTime() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aggregate.UpdatedAt.Unix()
}

// GetAggregate returns the underlying ThreadAggregate (for internal use).
func (s *SessionAdapter) GetAggregate() *domain.ThreadAggregate {
	return s.aggregate
}

// MapState implements agent.State using a map.
type MapState struct {
	data map[string]any
	mu   sync.RWMutex
}

// NewMapState creates a new MapState.
func NewMapState() *MapState {
	return &MapState{
		data: make(map[string]any),
	}
}

// Get retrieves a value by key.
func (s *MapState) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	if !ok {
		return nil, agent.ErrStateKeyNotFound
	}
	return val, nil
}

// Set stores a value by key.
func (s *MapState) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

// Delete removes a value by key.
func (s *MapState) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	return nil
}

// Keys returns all stored keys.
func (s *MapState) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// All returns an iterator over all key-value pairs.
func (s *MapState) All() func(yield func(key string, value any) bool) {
	return func(yield func(key string, value any) bool) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for k, v := range s.data {
			if !yield(k, v) {
				return
			}
		}
	}
}

// convertDomainMessageToAgent converts domain message to agent message.
func convertDomainMessageToAgent(msg domain.Message) *agent.Message {
	agentMsg := &agent.Message{
		ID:        msg.ID,
		Role:      agent.MessageRole(msg.Role),
		Content:   make([]agent.Content, 0),
		Timestamp: msg.Timestamp,
	}

	if agentMsg.Timestamp.IsZero() {
		agentMsg.Timestamp = time.Now()
	}

	// Convert content string
	if msg.Content != "" {
		agentMsg.Content = append(agentMsg.Content, agent.Content{
			Type: agent.ContentTypeText,
			Text: msg.Content,
		})
	}

	// Convert tool calls
	for _, tc := range msg.ToolCalls {
		agentMsg.Content = append(agentMsg.Content, agent.Content{
			Type: agent.ContentTypeToolCall,
			ToolCall: &agent.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})
	}

	// Convert tool result
	if msg.ToolCallID != "" {
		agentMsg.Content = append(agentMsg.Content, agent.Content{
			Type: agent.ContentTypeToolResult,
			ToolResult: &agent.ToolResult{
				ToolCallID: msg.ToolCallID,
				Output:     msg.Content,
				IsError:    false, // Default to false
			},
		})
	}

	return agentMsg
}

// convertAgentMessageToCore converts agent message to core message.
func convertAgentMessageToCore(msg *agent.Message) core.Message {
	coreMsg := core.Message{
		ID:        msg.ID,
		Role:      string(msg.Role),
		Timestamp: msg.Timestamp,
	}

	if coreMsg.Timestamp.IsZero() {
		coreMsg.Timestamp = time.Now()
	}

	// Convert content
	for _, c := range msg.Content {
		switch c.Type {
		case agent.ContentTypeText:
			coreMsg.Content = c.Text
		case agent.ContentTypeToolCall:
			if c.ToolCall != nil {
				coreMsg.ToolCalls = append(coreMsg.ToolCalls, core.ToolCall{
					ID:        c.ToolCall.ID,
					Type:      "function",
					Name:      c.ToolCall.Name,
					Arguments: c.ToolCall.Arguments,
				})
			}
		case agent.ContentTypeToolResult:
			if c.ToolResult != nil {
				coreMsg.ToolCallID = c.ToolResult.ToolCallID
				coreMsg.Content = c.ToolResult.Output
				// ToolName is handled by the higher level if needed,
				// but agent.Message doesn't have it on ToolResult yet.
				// However, core.Message does.
			}
		}
	}

	return coreMsg
}

// SessionStoreAdapter adapts domain.ThreadRepository to agent.SessionStore.
type SessionStoreAdapter struct {
	repo *domain.ThreadRepository
}

// NewSessionStoreAdapter creates a new session store adapter.
func NewSessionStoreAdapter(repo *domain.ThreadRepository) agent.SessionStore {
	return &SessionStoreAdapter{repo: repo}
}

// Get retrieves a session by ID.
func (s *SessionStoreAdapter) Get(ctx context.Context, appName, userID, sessionID string) (agent.Session, error) {
	aggregate, err := s.repo.Load(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	return NewSessionAdapter(aggregate), nil
}

// Create initializes a new session.
func (s *SessionStoreAdapter) Create(ctx context.Context, appName, userID string) (agent.Session, error) {
	aggregate := domain.NewThreadAggregate(core.GenerateID())
	aggregate.UserID = userID // NewThreadAggregate doesn't take userID now
	return NewSessionAdapter(aggregate), nil
}

// Save persists session changes.
func (s *SessionStoreAdapter) Save(ctx context.Context, session agent.Session) error {
	if sa, ok := session.(*SessionAdapter); ok {
		return s.repo.Save(ctx, sa.aggregate)
	}
	return fmt.Errorf("invalid session type")
}

// Delete removes a session.
func (s *SessionStoreAdapter) Delete(ctx context.Context, appName, userID, sessionID string) error {
	return s.repo.Delete(ctx, sessionID)
}

// List returns all sessions for a user.
func (s *SessionStoreAdapter) List(ctx context.Context, appName, userID string) ([]agent.Session, error) {
	// ThreadRepository doesn't have a List method, so return empty for now
	// TODO: Implement when ThreadRepository supports listing
	return nil, nil
}
