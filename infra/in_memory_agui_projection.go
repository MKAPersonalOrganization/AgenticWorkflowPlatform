package infra

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/services/agents"
	"school-with-ai/services/sdk"
)

// InMemoryAguiProjection is an in-memory implementation of the AguiProjection.
//
// This projection is optimized for UI queries and rebuilt from the EventStore.
// It maintains:
//   - Active message streams (for streaming UI updates)
//   - Tool call execution tracking
//   - Agent activity metrics
//   - Run lifecycle state
type InMemoryAguiProjection struct {
	mu sync.RWMutex

	// Thread state indexed by threadID
	threads map[string]*sdk.AguiThreadState

	// Active runs indexed by threadID
	activeRuns map[string]*sdk.AguiRunState

	// Active messages being streamed (messageID -> message)
	streamingMessages map[string]*sdk.AguiMessage

	// Tool calls indexed by threadID
	toolCallsByThread map[string]map[string]*sdk.AguiToolCall
}

// NewInMemoryAguiProjection creates a new in-memory AGUI projection.
func NewInMemoryAguiProjection() *InMemoryAguiProjection {
	return &InMemoryAguiProjection{
		threads:           make(map[string]*sdk.AguiThreadState),
		activeRuns:        make(map[string]*sdk.AguiRunState),
		streamingMessages: make(map[string]*sdk.AguiMessage),
		toolCallsByThread: make(map[string]map[string]*sdk.AguiToolCall),
	}
}

// Apply updates the projection with a single event.
func (p *InMemoryAguiProjection) Apply(ctx context.Context, event sdk.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	threadID := event.AggregateID()
	if threadID == "" {
		return nil // Skip events without thread context
	}

	// Ensure thread state exists
	if _, exists := p.threads[threadID]; !exists {
		p.threads[threadID] = &sdk.AguiThreadState{
			ThreadID:      threadID,
			Messages:      []sdk.AguiMessage{},
			ToolCalls:     []sdk.AguiToolCall{},
			AgentActivity: make(map[string]*sdk.AguiAgentActivity),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		p.toolCallsByThread[threadID] = make(map[string]*sdk.AguiToolCall)
	}

	thread := p.threads[threadID]
	thread.UpdatedAt = time.Now()
	thread.Version++

	// Apply event-specific logic
	switch evt := event.(type) {
	case *sdk.ThreadCreatedEvent:
		thread.Title = evt.Title
		thread.CreatedAt = evt.Timestamp

	case *sdk.UserMessageAddedEvent:
		p.addMessage(thread, sdk.AguiMessage{
			ID:        evt.MessageID,
			Role:      "user",
			Content:   evt.Content,
			Timestamp: evt.Timestamp,
			Status:    "complete",
		})

	case *sdk.AssistantMessageStartedEvent:
		msg := sdk.AguiMessage{
			ID:        evt.MessageID,
			Role:      "assistant",
			Content:   "",
			Timestamp: evt.Timestamp,
			Status:    "streaming",
		}
		p.addMessage(thread, msg)
		p.streamingMessages[evt.MessageID] = &thread.Messages[len(thread.Messages)-1]

	case *sdk.AssistantContentStreamedEvent:
		if msg, exists := p.streamingMessages[evt.MessageID]; exists {
			msg.Content += evt.Delta
		}

	case *sdk.AssistantMessageCompletedEvent:
		if msg, exists := p.streamingMessages[evt.MessageID]; exists {
			msg.Status = "complete"
			msg.Content = evt.Content // Final content
			if len(evt.ToolCalls) > 0 {
				msg.ToolCalls = make([]sdk.AguiMessageToolCall, len(evt.ToolCalls))
				for i, tc := range evt.ToolCalls {
					msg.ToolCalls[i] = sdk.AguiMessageToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					}
				}
			}
			delete(p.streamingMessages, evt.MessageID)
		}
		p.updateAgentActivity(thread, evt.AgentName, evt.Timestamp)

	case *sdk.ToolCallStartedEvent:
		toolCall := &sdk.AguiToolCall{
			ID:              evt.ToolCallID,
			Name:            evt.ToolCallName,
			Arguments:       "",
			AgentName:       evt.AgentName,
			ParentMessageID: evt.ParentMessageID,
			Status:          "started",
			StartedAt:       evt.Timestamp,
		}
		p.toolCallsByThread[threadID][evt.ToolCallID] = toolCall
		p.updateAgentActivity(thread, evt.AgentName, evt.Timestamp)

	case *sdk.ToolCallArgumentsStreamedEvent:
		if tc, exists := p.toolCallsByThread[threadID][evt.ToolCallID]; exists {
			tc.Arguments += evt.Delta
		}

	case *sdk.ToolCallCompletedEvent:
		if tc, exists := p.toolCallsByThread[threadID][evt.ToolCallID]; exists {
			tc.Status = "args_complete"
		}

	case *sdk.ToolCallExecutedEvent:
		msg := sdk.AguiMessage{
			ID:        evt.MessageID,
			Role:      "tool",
			Content:   evt.ToolResult,
			AgentName: evt.AgentName,
			Timestamp: evt.Timestamp,
			Status:    "complete",
			ToolCall: &sdk.AguiMessageToolResult{
				ID:     evt.ToolCallID,
				Result: evt.ToolResult,
			},
		}
		p.addMessage(thread, msg)

		// Update tool call to completed
		if tc, exists := p.toolCallsByThread[threadID][evt.ToolCallID]; exists {
			tc.Status = "completed"
			tc.Result = evt.ToolResult
			tc.CompletedAt = evt.Timestamp
			thread.ToolCalls = append(thread.ToolCalls, *tc)
		}
		p.updateAgentActivity(thread, evt.AgentName, evt.Timestamp)

	case *sdk.ThreadTitleUpdatedEvent:
		thread.Title = evt.NewTitle

	case *sdk.ThreadDeletedEvent:
		delete(p.threads, threadID)
		delete(p.activeRuns, threadID)
		delete(p.toolCallsByThread, threadID)

	// Handle custom domain events
	case *agents.DomainEvent:
		p.applyDomainEvent(thread, evt)
	}

	return nil
}

// applyDomainEvent handles custom domain events.
func (p *InMemoryAguiProjection) applyDomainEvent(thread *sdk.AguiThreadState, evt *agents.DomainEvent) {
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

	case agents.EventTypeAgentHandoff:
		msg := sdk.AguiMessage{
			ID:        fmt.Sprintf("handoff-%s-%d", evt.ThreadID, evt.Timestamp.UnixNano()),
			Role:      "system",
			Content:   fmt.Sprintf("Agent handoff: %s → %s", evt.GetString("from_agent"), evt.GetString("to_agent")),
			Timestamp: evt.Timestamp,
			Status:    "complete",
			Metadata: map[string]any{
				"type":       "agent_handoff",
				"from_agent": evt.GetString("from_agent"),
				"to_agent":   evt.GetString("to_agent"),
				"reason":     evt.GetString("reason"),
			},
		}
		p.addMessage(thread, msg)

	case agents.EventTypeMathQuestion,
		agents.EventTypePhysicsQuestion,
		agents.EventTypeChemistryQuestion,
		agents.EventTypeStatisticsQuestion,
		agents.EventTypeLinearAlgebraQuestion,
		agents.EventTypeDiagrammingQuestion:
		// Question events - could add to activity log if needed
		agentName := evt.GetString("emitted_by")
		if agentName != "" {
			p.updateAgentActivity(thread, agentName, evt.Timestamp)
		}

	case agents.EventTypeQuestionAnswered:
		// Answer events - track activity
		agentName := evt.GetString("answered_by")
		if agentName != "" {
			p.updateAgentActivity(thread, agentName, evt.Timestamp)
		}
	}
}

// addMessage adds a message to the thread state.
func (p *InMemoryAguiProjection) addMessage(thread *sdk.AguiThreadState, msg sdk.AguiMessage) {
	thread.Messages = append(thread.Messages, msg)
}

// updateAgentActivity updates the agent activity metrics.
func (p *InMemoryAguiProjection) updateAgentActivity(thread *sdk.AguiThreadState, agentName string, timestamp time.Time) {
	if agentName == "" {
		return
	}

	activity, exists := thread.AgentActivity[agentName]
	if !exists {
		activity = &sdk.AguiAgentActivity{
			AgentName:     agentName,
			FirstActiveAt: timestamp,
		}
		thread.AgentActivity[agentName] = activity
	}

	activity.LastActiveAt = timestamp
}

// GetThreadState retrieves the UI state for a thread.
func (p *InMemoryAguiProjection) GetThreadState(threadID string) (*sdk.AguiThreadState, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modifications
	threadCopy := *thread
	threadCopy.Messages = make([]sdk.AguiMessage, len(thread.Messages))
	copy(threadCopy.Messages, thread.Messages)
	threadCopy.ToolCalls = make([]sdk.AguiToolCall, len(thread.ToolCalls))
	copy(threadCopy.ToolCalls, thread.ToolCalls)

	return &threadCopy, true
}

// GetActiveRun retrieves information about the currently active run for a thread.
func (p *InMemoryAguiProjection) GetActiveRun(threadID string) (*sdk.AguiRunState, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	run, exists := p.activeRuns[threadID]
	if !exists {
		return nil, false
	}

	runCopy := *run
	return &runCopy, true
}

// GetMessages retrieves all UI-formatted messages for a thread.
func (p *InMemoryAguiProjection) GetMessages(threadID string) []sdk.AguiMessage {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return []sdk.AguiMessage{}
	}

	messages := make([]sdk.AguiMessage, len(thread.Messages))
	copy(messages, thread.Messages)
	return messages
}

// GetToolCalls retrieves all tool calls for a thread.
func (p *InMemoryAguiProjection) GetToolCalls(threadID string) []sdk.AguiToolCall {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return []sdk.AguiToolCall{}
	}

	toolCalls := make([]sdk.AguiToolCall, len(thread.ToolCalls))
	copy(toolCalls, thread.ToolCalls)
	return toolCalls
}

// GetAgentActivity retrieves agent activity summary for a thread.
func (p *InMemoryAguiProjection) GetAgentActivity(threadID string) map[string]*sdk.AguiAgentActivity {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return make(map[string]*sdk.AguiAgentActivity)
	}

	// Return a copy
	activity := make(map[string]*sdk.AguiAgentActivity, len(thread.AgentActivity))
	for name, act := range thread.AgentActivity {
		actCopy := *act
		activity[name] = &actCopy
	}
	return activity
}

// ListThreads returns all thread IDs in the projection.
func (p *InMemoryAguiProjection) ListThreads() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	threads := make([]string, 0, len(p.threads))
	for threadID := range p.threads {
		threads = append(threads, threadID)
	}
	return threads
}

// Rebuild reconstructs the projection from the EventStore for a single stream or all streams.
// If streamID is empty, it rebuilds all streams.
func (p *InMemoryAguiProjection) Rebuild(ctx context.Context, store sdk.EventStore, streamID string) error {
	var streamIDs []string
	var err error

	if streamID == "" {
		// Rebuild all streams
		streamIDs, err = store.ListStreams(ctx)
		if err != nil {
			return fmt.Errorf("failed to list streams: %w", err)
		}

		p.mu.Lock()
		// Clear all state
		p.threads = make(map[string]*sdk.AguiThreadState)
		p.activeRuns = make(map[string]*sdk.AguiRunState)
		p.streamingMessages = make(map[string]*sdk.AguiMessage)
		p.toolCallsByThread = make(map[string]map[string]*sdk.AguiToolCall)
		p.mu.Unlock()
	} else {
		// Rebuild single stream
		streamIDs = []string{streamID}

		p.mu.Lock()
		// Clear state for this stream
		delete(p.threads, streamID)
		delete(p.activeRuns, streamID)
		delete(p.toolCallsByThread, streamID)
		p.mu.Unlock()
	}

	// Replay all events for each stream
	for _, sid := range streamIDs {
		events, err := store.ReadAll(ctx, sid)
		if err != nil {
			return fmt.Errorf("failed to read events for stream %s: %w", sid, err)
		}

		for _, event := range events {
			if err := p.Apply(ctx, event); err != nil {
				return fmt.Errorf("failed to apply event during rebuild: %w", err)
			}
		}
	}

	return nil
}

// Name returns the projection name.
func (p *InMemoryAguiProjection) Name() string {
	return "agui-projection"
}

// Clear removes all state (useful for testing).
func (p *InMemoryAguiProjection) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.threads = make(map[string]*sdk.AguiThreadState)
	p.activeRuns = make(map[string]*sdk.AguiRunState)
	p.streamingMessages = make(map[string]*sdk.AguiMessage)
	p.toolCallsByThread = make(map[string]map[string]*sdk.AguiToolCall)
}

// Verify InMemoryAguiProjection implements AguiProjection
var _ sdk.AguiProjection = (*InMemoryAguiProjection)(nil)
