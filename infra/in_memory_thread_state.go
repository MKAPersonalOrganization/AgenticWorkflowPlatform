package infra

import (
	"fmt"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/sdk"
)

// InMemoryThreadState is an in-memory implementation of ThreadState
type InMemoryThreadState struct {
	mu      sync.RWMutex
	threads map[string]sdk.AgentThread
	// Metadata per thread (title, timestamps, etc.)
	metadata map[string]*threadMetadata
}

type threadMetadata struct {
	Title             string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CurrentInstructor string
	InstructorHistory []domain.InstructorSession
	WhiteboardContent string
}

// NewInMemoryThreadState creates a new in-memory thread state
func NewInMemoryThreadState() *InMemoryThreadState {
	return &InMemoryThreadState{
		threads:  make(map[string]sdk.AgentThread),
		metadata: make(map[string]*threadMetadata),
	}
}

// Get retrieves a thread by ID
func (p *InMemoryThreadState) Get(threadID string) (*domain.Thread, bool) {
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

	// Convert AgentThread to domain.Thread for the projection
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

// GetAgentThread retrieves the underlying AgentThread by ID
func (p *InMemoryThreadState) GetAgentThread(threadID string) (sdk.AgentThread, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	agentThread, exists := p.threads[threadID]
	return agentThread, exists
}

// Messages retrieves all messages from a thread
func (p *InMemoryThreadState) Messages(threadID string) []domain.Message {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if agentThread, exists := p.threads[threadID]; exists {
		return agentThread.GetMessages()
	}
	return []domain.Message{}
}

// All retrieves all threads sorted by UpdatedAt (newest first)
func (p *InMemoryThreadState) All() []domain.Thread {
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

	// Get or create thread and metadata
	agentThread, exists := p.threads[threadID]
	if !exists {
		agentThread = sdk.NewInMemoryThreadWithID(threadID)
		p.threads[threadID] = agentThread
		p.metadata[threadID] = &threadMetadata{}
	}

	meta := p.metadata[threadID]
	return p.applyEventUnsafe(agentThread, meta, event)
}

// applyEventUnsafe applies an event to a thread (caller must hold lock)
func (p *InMemoryThreadState) applyEventUnsafe(agentThread sdk.AgentThread, meta *threadMetadata, event sdk.Event) error {
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
		// Marker event for traceability - no state change
		meta.UpdatedAt = evt.Timestamp

	case *sdk.AssistantContentStreamedEvent:
		// Ephemeral streaming event - ignore in projection
		// Only used for real-time UI updates

	case *sdk.ToolCallStartedEvent:
		// Ephemeral event - agent info preserved in event stream order

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
		delete(p.threads, agentThread.GetID())
		delete(p.metadata, agentThread.GetID())

	// Agent collaboration events
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
			// Store agent handoff as a system message
			// Content format: "AGENT_HANDOFF|from|to|reason" for parsing on frontend
			msgID := fmt.Sprintf("handoff-%s-%d", evt.ThreadID, evt.Timestamp.UnixNano())

			// Check if this handoff message already exists (deduplication)
			messages := agentThread.GetMessages()
			exists := false
			for _, msg := range messages {
				if msg.ID == msgID {
					exists = true
					break
				}
			}

			if !exists {
				msg := domain.Message{
					ID:        msgID,
					Role:      "system",
					Content:   "AGENT_HANDOFF|" + evt.GetString("from_agent") + "|" + evt.GetString("to_agent") + "|" + evt.GetString("reason"),
					Timestamp: evt.Timestamp,
					Metadata: map[string]interface{}{
						"type":       "agent_handoff",
						"from_agent": evt.GetString("from_agent"),
						"to_agent":   evt.GetString("to_agent"),
						"reason":     evt.GetString("reason"),
					},
				}
				agentThread.AddMessages([]domain.Message{msg})
			}
			meta.UpdatedAt = evt.Timestamp
		}

	case *sdk.CustomEventEmittedEvent:
		meta.UpdatedAt = evt.Timestamp
	}

	return nil
}
