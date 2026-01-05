package llmcontext

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/sdk"
)

// InMemoryLLMContextProjection is an in-memory projection of thread state for LLM context.
//
// IMPORTANT: This is a READ MODEL (projection), not a primary storage.
// In event sourcing:
//   - EventStore is the source of truth (append-only events)
//   - InMemoryLLMContextProjection is a projection (derived from events)
//   - Agents READ from this projection to get conversation context
//   - Agents WRITE by emitting events to EventStore
//
// This projection:
//   - Is updated by applying events via Update()
//   - Can be rebuilt from EventStore at any time
//   - Provides optimized read access for LLM agents
type InMemoryLLMContextProjection struct {
	mu sync.RWMutex

	// Thread conversation state (messages)
	threads map[string]sdk.AgentThread

	// Thread metadata (title, timestamps, etc.)
	// This is derived from events, not stored separately
	metadata map[string]*threadMetadata
}

type threadMetadata struct {
	Title             string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CurrentInstructor string
	InstructorHistory []domain.InstructorSession
	WhiteboardContent string
	Version           int64 // Event version this projection is at
}

// NewInMemoryLLMContextProjection creates a new in-memory LLM context projection.
func NewInMemoryLLMContextProjection() *InMemoryLLMContextProjection {
	return &InMemoryLLMContextProjection{
		threads:  make(map[string]sdk.AgentThread),
		metadata: make(map[string]*threadMetadata),
	}
}

// Get retrieves a thread projection by ID.
func (p *InMemoryLLMContextProjection) Get(threadID string) (*domain.Thread, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	agentThread, exists := p.threads[threadID]
	if !exists {
		return nil, false
	}

	meta := p.metadata[threadID]
	if meta == nil {
		meta = &threadMetadata{}
	}

	// Build the read model from projection state
	thread := &domain.Thread{
		ID:                agentThread.GetID(),
		Title:             meta.Title,
		Messages:          agentThread.GetMessages(),
		CreatedAt:         meta.CreatedAt,
		UpdatedAt:         meta.UpdatedAt,
		CurrentInstructor: meta.CurrentInstructor,
		InstructorHistory: meta.InstructorHistory,
		WhiteboardContent: meta.WhiteboardContent,
	}

	return thread, true
}

// GetAgentThread retrieves the conversation state for agent processing.
func (p *InMemoryLLMContextProjection) GetAgentThread(threadID string) (sdk.AgentThread, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	agentThread, exists := p.threads[threadID]
	return agentThread, exists
}

// GetOrCreateAgentThread ensures a thread exists in the projection.
// Note: This doesn't write to EventStore - it just ensures the projection
// is ready to receive events for this thread.
func (p *InMemoryLLMContextProjection) GetOrCreateAgentThread(ctx context.Context, threadID string) (sdk.AgentThread, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if agentThread, exists := p.threads[threadID]; exists {
		return agentThread, nil
	}

	// Create empty projection entry (will be populated by events)
	agentThread := sdk.NewInMemoryThreadWithID(threadID)
	p.threads[threadID] = agentThread
	p.metadata[threadID] = &threadMetadata{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return agentThread, nil
}

// Messages retrieves all messages from a thread's projection.
func (p *InMemoryLLMContextProjection) Messages(threadID string) []domain.Message {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if agentThread, exists := p.threads[threadID]; exists {
		return agentThread.GetMessages()
	}
	return []domain.Message{}
}

// All retrieves all thread projections sorted by UpdatedAt (newest first).
func (p *InMemoryLLMContextProjection) All() []domain.Thread {
	p.mu.RLock()
	defer p.mu.RUnlock()

	threads := make([]domain.Thread, 0, len(p.threads))
	for threadID, agentThread := range p.threads {
		meta := p.metadata[threadID]
		if meta == nil {
			meta = &threadMetadata{}
		}

		thread := domain.Thread{
			ID:                agentThread.GetID(),
			Title:             meta.Title,
			Messages:          agentThread.GetMessages(),
			CreatedAt:         meta.CreatedAt,
			UpdatedAt:         meta.UpdatedAt,
			CurrentInstructor: meta.CurrentInstructor,
			InstructorHistory: meta.InstructorHistory,
			WhiteboardContent: meta.WhiteboardContent,
		}
		threads = append(threads, thread)
	}

	// Sort by UpdatedAt descending
	for i := 0; i < len(threads)-1; i++ {
		for j := i + 1; j < len(threads); j++ {
			if threads[i].UpdatedAt.Before(threads[j].UpdatedAt) {
				threads[i], threads[j] = threads[j], threads[i]
			}
		}
	}

	return threads
}

// Apply updates the projection with a single event (implements sdk.Projection).
// This delegates to Update for backward compatibility.
func (p *InMemoryLLMContextProjection) Apply(ctx context.Context, event sdk.Event) error {
	return p.Update(event)
}

// Update applies a single event to update the projection.
// This is called when new events are appended to the EventStore.
func (p *InMemoryLLMContextProjection) Update(event sdk.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	threadID := event.AggregateID()

	// Ensure thread exists in projection
	agentThread, exists := p.threads[threadID]
	if !exists {
		agentThread = sdk.NewInMemoryThreadWithID(threadID)
		p.threads[threadID] = agentThread
		p.metadata[threadID] = &threadMetadata{}
	}

	meta := p.metadata[threadID]
	return p.applyEventUnsafe(agentThread, meta, event)
}

// Rebuild reconstructs the projection from the EventStore for a single thread.
func (p *InMemoryLLMContextProjection) Rebuild(ctx context.Context, store sdk.EventStore, threadID string) error {
	// Read all events for this thread
	events, err := store.ReadAll(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to read events for thread %s: %w", threadID, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear existing projection for this thread
	delete(p.threads, threadID)
	delete(p.metadata, threadID)

	if len(events) == 0 {
		return nil
	}

	// Create fresh projection entry
	agentThread := sdk.NewInMemoryThreadWithID(threadID)
	p.threads[threadID] = agentThread
	p.metadata[threadID] = &threadMetadata{}
	meta := p.metadata[threadID]

	// Replay all events
	for _, event := range events {
		if err := p.applyEventUnsafe(agentThread, meta, event); err != nil {
			return fmt.Errorf("failed to apply event during rebuild: %w", err)
		}
	}

	meta.Version = int64(len(events))
	return nil
}

// RebuildAll reconstructs all thread projections from the EventStore.
func (p *InMemoryLLMContextProjection) RebuildAll(ctx context.Context, store sdk.EventStore) error {
	// List all streams
	streamIDs, err := store.ListStreams(ctx)
	if err != nil {
		return fmt.Errorf("failed to list streams: %w", err)
	}

	p.mu.Lock()
	// Clear all projections
	p.threads = make(map[string]sdk.AgentThread)
	p.metadata = make(map[string]*threadMetadata)
	p.mu.Unlock()

	// Rebuild each thread
	for _, threadID := range streamIDs {
		if err := p.Rebuild(ctx, store, threadID); err != nil {
			return err
		}
	}

	return nil
}

// applyEventUnsafe applies an event to a thread (caller must hold lock).
func (p *InMemoryLLMContextProjection) applyEventUnsafe(agentThread sdk.AgentThread, meta *threadMetadata, event sdk.Event) error {
	switch evt := event.(type) {
	case *sdk.ThreadCreatedEvent:
		meta.Title = evt.Title
		meta.CreatedAt = evt.Timestamp
		meta.UpdatedAt = evt.Timestamp

	case *sdk.UserMessageAddedEvent:
		msg := domain.Message{
			ID:        evt.MessageID,
			Role:      "user",
			Content:   evt.Content,
			Timestamp: evt.Timestamp,
		}
		agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp

	case *sdk.AssistantMessageStartedEvent:
		meta.UpdatedAt = evt.Timestamp

	case *sdk.AssistantContentStreamedEvent:
		// Ephemeral streaming event - ignore in projection

	case *sdk.ToolCallStartedEvent:
		// Ephemeral event

	case *sdk.ToolCallArgumentsStreamedEvent:
		// Ephemeral streaming event

	case *sdk.ToolCallCompletedEvent:
		// Ephemeral streaming event

	case *sdk.AssistantMessageCompletedEvent:
		msg := domain.Message{
			ID:        evt.MessageID,
			Role:      "assistant",
			Content:   evt.Content,
			ToolCalls: evt.ToolCalls,
			Timestamp: evt.Timestamp,
			AgentName: evt.AgentName,
		}
		agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp

	case *sdk.ToolCallExecutedEvent:
		msg := domain.Message{
			ID:         evt.MessageID,
			Role:       "tool",
			Content:    evt.ToolResult,
			ToolCallID: evt.ToolCallID,
			Timestamp:  evt.Timestamp,
			AgentName:  evt.AgentName,
		}
		agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp

	case *sdk.ThreadTitleUpdatedEvent:
		meta.Title = evt.NewTitle
		meta.UpdatedAt = evt.Timestamp

	case *sdk.ThreadDeletedEvent:
		threadID := agentThread.GetID()
		delete(p.threads, threadID)
		delete(p.metadata, threadID)

	case *agents.DomainEvent:
		switch evt.Type {
		case agents.EventTypeWhiteboardUpdated:
			action := evt.GetString("action")
			content := evt.GetString("content")
			switch action {
			case "write":
				meta.WhiteboardContent = content
			case "append":
				meta.WhiteboardContent += "\n" + content
			case "clear":
				meta.WhiteboardContent = ""
			}
			meta.UpdatedAt = evt.Timestamp
		case agents.EventTypeAgentHandoff:
			msgID := fmt.Sprintf("handoff-%s-%d", evt.ThreadID, evt.Timestamp.UnixNano())

			// Check for duplicate
			messages := agentThread.GetMessages()
			for _, msg := range messages {
				if msg.ID == msgID {
					return nil // Already applied
				}
			}

			msg := domain.Message{
				ID:        msgID,
				Role:      "system",
				Content:   "AGENT_HANDOFF|" + evt.GetString("from_agent") + "|" + evt.GetString("to_agent") + "|" + evt.GetString("reason"),
				Timestamp: evt.Timestamp,
				Metadata: map[string]any{
					"type":       "agent_handoff",
					"from_agent": evt.GetString("from_agent"),
					"to_agent":   evt.GetString("to_agent"),
					"reason":     evt.GetString("reason"),
				},
			}
			agentThread.AddMessages([]domain.Message{msg})
			meta.UpdatedAt = evt.Timestamp
		}

	case *sdk.CustomEventEmittedEvent:
		meta.UpdatedAt = evt.Timestamp
	}

	meta.Version++
	return nil
}

// ThreadExists checks if a thread has any events in the projection.
func (p *InMemoryLLMContextProjection) ThreadExists(threadID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.threads[threadID]
	return exists
}

// Clear removes all projections (useful for testing).
func (p *InMemoryLLMContextProjection) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.threads = make(map[string]sdk.AgentThread)
	p.metadata = make(map[string]*threadMetadata)
}

// Verify InMemoryLLMContextProjection implements ThreadStateWithAgentAccess
var _ sdk.ThreadStateWithAgentAccess = (*InMemoryLLMContextProjection)(nil)
