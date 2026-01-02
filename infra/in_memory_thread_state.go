package infra

import (
	"sync"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/sdk"
)

// InMemoryThreadState is an in-memory implementation of ThreadState
type InMemoryThreadState struct {
	mu      sync.RWMutex
	threads map[string]*domain.Thread
}

// NewInMemoryThreadState creates a new in-memory thread state
func NewInMemoryThreadState() *InMemoryThreadState {
	return &InMemoryThreadState{
		threads: make(map[string]*domain.Thread),
	}
}

// Get retrieves a thread by ID
func (p *InMemoryThreadState) Get(threadID string) (*domain.Thread, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	return thread, exists
}

// Messages retrieves all messages from a thread
func (p *InMemoryThreadState) Messages(threadID string) []domain.Message {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if thread, exists := p.threads[threadID]; exists {
		return thread.Messages
	}
	return []domain.Message{}
}

// All retrieves all threads sorted by UpdatedAt (newest first)
func (p *InMemoryThreadState) All() []domain.Thread {
	p.mu.RLock()
	defer p.mu.RUnlock()

	threads := make([]domain.Thread, 0, len(p.threads))
	for _, thread := range p.threads {
		threads = append(threads, *thread)
	}

	// Simple bubble sort by UpdatedAt descending
	for i := 0; i < len(threads)-1; i++ {
		for j := i + 1; j < len(threads); j++ {
			if threads[i].UpdatedAt.Before(threads[j].UpdatedAt) {
				threads[i], threads[j] = threads[j], threads[i]
			}
		}
	}

	return threads
}

// Update applies a single event to update the state
func (p *InMemoryThreadState) Update(event sdk.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	threadID := event.AggregateID()

	// Get or create thread
	thread, exists := p.threads[threadID]
	if !exists {
		thread = &domain.Thread{
			ID:       threadID,
			Messages: []domain.Message{},
		}
		p.threads[threadID] = thread
	}

	return p.applyEventUnsafe(thread, event)
}

// applyEventUnsafe applies an event to a thread (caller must hold lock)
func (p *InMemoryThreadState) applyEventUnsafe(thread *domain.Thread, event sdk.Event) error {
	switch evt := event.(type) {
	case *sdk.ThreadCreatedEvent:
		thread.Title = evt.Title
		thread.CreatedAt = evt.Timestamp
		thread.UpdatedAt = evt.Timestamp

	case *sdk.UserMessageAddedEvent:
		msg := domain.Message{
			ID:        evt.MessageID,
			Role:      "user",
			Content:   evt.Content,
			Timestamp: evt.Timestamp,
		}
		thread.Messages = append(thread.Messages, msg)
		thread.UpdatedAt = evt.Timestamp

	case *sdk.AssistantMessageStartedEvent:
		// Marker event for traceability - no state change
		thread.UpdatedAt = evt.Timestamp

	case *sdk.AssistantContentStreamedEvent:
		// Ephemeral streaming event - ignore in projection
		// Only used for real-time UI updates

	case *sdk.ToolCallStartedEvent:
		// Ephemeral streaming event - ignore in projection

	case *sdk.ToolCallArgumentsStreamedEvent:
		// Ephemeral streaming event - ignore in projection

	case *sdk.ToolCallCompletedEvent:
		// Ephemeral streaming event - ignore in projection

	case *sdk.AssistantMessageCompletedEvent:
		msg := domain.Message{
			ID:        evt.MessageID,
			Role:      "assistant",
			Content:   evt.Content,
			ToolCalls: evt.ToolCalls,
			Timestamp: evt.Timestamp,
			AgentName: evt.AgentName,
		}
		thread.Messages = append(thread.Messages, msg)
		thread.UpdatedAt = evt.Timestamp

	case *sdk.ToolCallExecutedEvent:
		msg := domain.Message{
			ID:         evt.MessageID,
			Role:       "tool",
			Content:    evt.ToolResult,
			ToolCallID: evt.ToolCallID,
			Timestamp:  evt.Timestamp,
			AgentName:  evt.AgentName,
		}
		thread.Messages = append(thread.Messages, msg)
		thread.UpdatedAt = evt.Timestamp

	case *sdk.ThreadTitleUpdatedEvent:
		thread.Title = evt.NewTitle
		thread.UpdatedAt = evt.Timestamp

	case *sdk.ThreadDeletedEvent:
		delete(p.threads, thread.ID)

	// Agent collaboration events
	case *agents.DomainEvent:
		switch evt.Type {
		case agents.EventTypeWhiteboardUpdated:
			action := evt.GetString("action")
			content := evt.GetString("content")
			switch action {
			case "write":
				thread.WhiteboardContent = content
			case "append":
				thread.WhiteboardContent += "\n" + content
			case "clear":
				thread.WhiteboardContent = ""
			}
			thread.UpdatedAt = evt.Timestamp
		}

	case *sdk.CustomEventEmittedEvent:
		thread.UpdatedAt = evt.Timestamp
	}

	return nil
}
