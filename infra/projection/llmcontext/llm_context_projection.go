package llmcontext

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
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
	threads map[string]core.AgentThread

	// Thread metadata (title, timestamps, etc.)
	// This is derived from events, not stored separately
	metadata map[string]*threadMetadata
}

type threadMetadata struct {
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
	Version   int64 // Event version this projection is at
}

// NewInMemoryLLMContextProjection creates a new in-memory LLM context projection.
func NewInMemoryLLMContextProjection() *InMemoryLLMContextProjection {
	return &InMemoryLLMContextProjection{
		threads:  make(map[string]core.AgentThread),
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
		ID:        agentThread.GetID(),
		Title:     meta.Title,
		Messages:  agentThread.GetMessages(),
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
	}

	return thread, true
}

// GetAgentThread retrieves the conversation state for agent processing.
func (p *InMemoryLLMContextProjection) GetAgentThread(threadID string) (core.AgentThread, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	agentThread, exists := p.threads[threadID]
	return agentThread, exists
}

// GetOrCreateAgentThread ensures a thread exists in the projection.
// Note: This doesn't write to EventStore - it just ensures the projection
// is ready to receive events for this thread.
func (p *InMemoryLLMContextProjection) GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if agentThread, exists := p.threads[threadID]; exists {
		return agentThread, nil
	}

	// Create empty projection entry (will be populated by events)
	agentThread := core.NewInMemoryThreadWithID(threadID)
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
			ID:        agentThread.GetID(),
			Title:     meta.Title,
			Messages:  agentThread.GetMessages(),
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
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

// Apply updates the projection with a single event (implements eventstore.Projection).
// This delegates to Update for backward compatibility.
func (p *InMemoryLLMContextProjection) Apply(ctx context.Context, event core.Event) error {
	return p.Update(event)
}

// Update applies a single event to update the projection.
// This is called when new events are appended to the EventStore.
func (p *InMemoryLLMContextProjection) Update(event core.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	threadID := event.AggregateID()

	// Ensure thread exists in projection
	agentThread, exists := p.threads[threadID]
	if !exists {
		agentThread = core.NewInMemoryThreadWithID(threadID)
		p.threads[threadID] = agentThread
		p.metadata[threadID] = &threadMetadata{}
	}

	meta := p.metadata[threadID]
	return p.applyEventUnsafe(agentThread, meta, event)
}

// Rebuild reconstructs the projection from the EventStore for a single thread.
func (p *InMemoryLLMContextProjection) Rebuild(ctx context.Context, store core.EventStore, threadID string) error {
	// Read all events for this thread
	events, err := store.ReadStream(ctx, threadID, 0)
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
	agentThread := core.NewInMemoryThreadWithID(threadID)
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
func (p *InMemoryLLMContextProjection) RebuildAll(ctx context.Context, store core.EventStore) error {
	// Since we don't have ListStreams anymore, we can't rebuild all
	// This would need to be implemented differently or removed
	return fmt.Errorf("RebuildAll not supported with minimal EventStore interface")
}

// applyEventUnsafe applies an event to a thread (caller must hold lock).
func (p *InMemoryLLMContextProjection) applyEventUnsafe(agentThread core.AgentThread, meta *threadMetadata, event core.Event) error {
	switch evt := event.(type) {
	case *core.ThreadCreatedEvent:
		meta.Title = evt.Title
		meta.CreatedAt = evt.Timestamp
		meta.UpdatedAt = evt.Timestamp

	case *core.UserMessageAddedEvent:
		msg := domain.Message{
			ID:        evt.MessageID,
			Role:      "user",
			Content:   evt.Content,
			Timestamp: evt.Timestamp,
		}
		_ = agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp

	case *core.AssistantMessageStartedEvent:
		meta.UpdatedAt = evt.Timestamp

	case *core.AssistantContentStreamedEvent:
		// Ephemeral streaming event - ignore in projection

	case *core.ToolCallStartedEvent:
		// Ephemeral event

	case *core.ToolCallArgumentsStreamedEvent:
		// Ephemeral streaming event

	case *core.ToolCallCompletedEvent:
		// Ephemeral streaming event

	case *core.AssistantMessageCompletedEvent:
		msg := domain.Message{
			ID:        evt.MessageID,
			Role:      "assistant",
			Content:   evt.Content,
			ToolCalls: evt.ToolCalls,
			Timestamp: evt.Timestamp,
			AgentName: evt.AgentName,
		}
		_ = agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp

	case *core.ToolCallExecutedEvent:
		msg := domain.Message{
			ID:         evt.MessageID,
			Role:       "tool",
			Content:    evt.ToolResult,
			ToolCallID: evt.ToolCallID,
			Timestamp:  evt.Timestamp,
			AgentName:  evt.AgentName,
		}
		_ = agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp

	case *core.ThreadTitleUpdatedEvent:
		meta.Title = evt.NewTitle
		meta.UpdatedAt = evt.Timestamp

	case *core.ThreadDeletedEvent:
		threadID := agentThread.GetID()
		delete(p.threads, threadID)
		delete(p.metadata, threadID)
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
	p.threads = make(map[string]core.AgentThread)
	p.metadata = make(map[string]*threadMetadata)
}

// Verify InMemoryLLMContextProjection implements ThreadStateWithAgentAccess
var _ core.ThreadStateWithAgentAccess = (*InMemoryLLMContextProjection)(nil)
