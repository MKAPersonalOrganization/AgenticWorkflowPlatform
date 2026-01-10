package querytime

import (
	"context"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
)

// ThreadLister provides a way to list all thread IDs.
type ThreadLister interface {
	ListThreads() []string
}

// QueryTimeLLMContextProjection is a query-time projection that rebuilds thread state
// from events on every read. This eliminates the need for in-memory state management.
//
// Benefits:
//   - No memory overhead for storing thread state
//   - Always consistent with event store (no projection lag)
//   - Simpler architecture - no need to manage projection updates
//   - Suitable for read-heavy workloads where event streams are small
type QueryTimeLLMContextProjection struct {
	eventStore   core.EventStore
	threadLister ThreadLister // Used to get list of all threads for All() method
}

// threadMetadata holds non-message thread information.
type threadMetadata struct {
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewQueryTimeLLMContextProjection creates a new query-time LLM context projection.
func NewQueryTimeLLMContextProjection(eventStore core.EventStore, threadLister ThreadLister) *QueryTimeLLMContextProjection {
	return &QueryTimeLLMContextProjection{
		eventStore:   eventStore,
		threadLister: threadLister,
	}
}

// Get retrieves a thread by reading and replaying events from the event store.
func (p *QueryTimeLLMContextProjection) Get(threadID string) (*domain.Thread, bool) {
	ctx := context.Background()

	// Read all events for this thread (using aggregate stream naming)
	streamName := domain.StreamName("Thread", threadID)
	events, err := p.eventStore.ReadStream(ctx, streamName, 0)
	if err != nil || len(events) == 0 {
		return nil, false
	}

	// Build thread state from events
	agentThread := core.NewInMemoryThreadWithID(threadID)
	meta := &threadMetadata{}

	for _, event := range events {
		_ = applyEventToThread(agentThread, meta, event)
	}

	// Build the read model
	thread := &domain.Thread{
		ID:        agentThread.GetID(),
		Title:     meta.Title,
		Messages:  agentThread.GetMessages(),
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
	}

	return thread, true
}

// GetAgentThread retrieves the conversation state for agent processing by rebuilding from events.
func (p *QueryTimeLLMContextProjection) GetAgentThread(threadID string) (core.AgentThread, bool) {
	ctx := context.Background()

	// Read all events for this thread (using aggregate stream naming)
	streamName := domain.StreamName("Thread", threadID)
	events, err := p.eventStore.ReadStream(ctx, streamName, 0)
	if err != nil || len(events) == 0 {
		return nil, false
	}

	// Rebuild thread from events
	agentThread := core.NewInMemoryThreadWithID(threadID)
	meta := &threadMetadata{}

	for _, event := range events {
		_ = applyEventToThread(agentThread, meta, event)
	}

	return agentThread, true
}

// GetOrCreateAgentThread ensures a thread exists, creating an empty one if needed.
func (p *QueryTimeLLMContextProjection) GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error) {
	// Try to get existing thread first
	if agentThread, exists := p.GetAgentThread(threadID); exists {
		return agentThread, nil
	}

	// Return empty thread (will be populated by events)
	return core.NewInMemoryThreadWithID(threadID), nil
}

// Messages retrieves all messages from a thread by rebuilding from events.
func (p *QueryTimeLLMContextProjection) Messages(threadID string) []domain.Message {
	if thread, exists := p.Get(threadID); exists {
		return thread.Messages
	}

	return []domain.Message{}
}

// All retrieves all threads by using the thread lister and then rebuilding each thread from events.
func (p *QueryTimeLLMContextProjection) All() []domain.Thread {
	if p.threadLister == nil {
		return []domain.Thread{}
	}

	threadIDs := p.threadLister.ListThreads()
	threads := make([]domain.Thread, 0, len(threadIDs))

	for _, threadID := range threadIDs {
		if thread, exists := p.Get(threadID); exists {
			threads = append(threads, *thread)
		}
	}

	// Sort by UpdatedAt descending
	for i := range len(threads) - 1 {
		for j := i + 1; j < len(threads); j++ {
			if threads[i].UpdatedAt.Before(threads[j].UpdatedAt) {
				threads[i], threads[j] = threads[j], threads[i]
			}
		}
	}

	return threads
}

// Update is a no-op for query-time projections since we read directly from the event store.
func (p *QueryTimeLLMContextProjection) Update(event core.Event) error {
	// No-op: query-time projections don't maintain state
	return nil
}

// Apply is a no-op for query-time projections.
func (p *QueryTimeLLMContextProjection) Apply(ctx context.Context, event core.Event) error {
	// No-op: query-time projections don't maintain state
	return nil
}

// ThreadExists checks if a thread has events in the event store.
func (p *QueryTimeLLMContextProjection) ThreadExists(threadID string) bool {
	ctx := context.Background()
	events, err := p.eventStore.ReadStream(ctx, threadID, 0)

	return err == nil && len(events) > 0
}

// Rebuild is a no-op for query-time projections since we always read fresh from the event store.
func (p *QueryTimeLLMContextProjection) Rebuild(ctx context.Context, store core.EventStore, threadID string) error {
	// No-op: query-time projections don't maintain state, so nothing to rebuild
	return nil
}

// RebuildAll is a no-op for query-time projections.
func (p *QueryTimeLLMContextProjection) RebuildAll(ctx context.Context, store core.EventStore) error {
	// No-op: query-time projections don't maintain state, so nothing to rebuild
	return nil
}

// applyEventToThread applies an event to rebuild thread state.
func applyEventToThread(agentThread core.AgentThread, meta *threadMetadata, event core.Event) error {
	switch evt := event.(type) {
	// Domain events (persisted in event store)
	case domain.ThreadCreated:
		meta.Title = evt.Title
		meta.CreatedAt = evt.Timestamp
		meta.UpdatedAt = evt.Timestamp
	case *domain.ThreadCreated:
		meta.Title = evt.Title
		meta.CreatedAt = evt.Timestamp
		meta.UpdatedAt = evt.Timestamp

	case domain.MessageAdded:
		msg := domain.Message{
			ID:         evt.MessageID,
			Role:       evt.Role,
			Content:    evt.Content,
			ToolCalls:  evt.ToolCalls,
			ToolCallID: evt.ToolCallID,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
			Metadata:   evt.Metadata,
		}
		_ = agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp
	case *domain.MessageAdded:
		msg := domain.Message{
			ID:         evt.MessageID,
			Role:       evt.Role,
			Content:    evt.Content,
			ToolCalls:  evt.ToolCalls,
			ToolCallID: evt.ToolCallID,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
			Metadata:   evt.Metadata,
		}
		_ = agentThread.AddMessages([]domain.Message{msg})
		meta.UpdatedAt = evt.Timestamp

	case domain.ThreadTitleChanged:
		meta.Title = evt.NewTitle
		meta.UpdatedAt = evt.Timestamp
	case *domain.ThreadTitleChanged:
		meta.Title = evt.NewTitle
		meta.UpdatedAt = evt.Timestamp

	case domain.ThreadDeleted:
		// Thread is deleted - caller should handle this
	case *domain.ThreadDeleted:
		// Thread is deleted - caller should handle this
	}

	return nil
}

// Verify QueryTimeLLMContextProjection implements core.ThreadStateWithAgentAccess.
var _ core.ThreadStateWithAgentAccess = (*QueryTimeLLMContextProjection)(nil)
