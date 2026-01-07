package agui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/sdk/agentsdk/core"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// EventEmitter is a callback for emitting AGUI events in real-time.
type EventEmitter func(threadID string, event interface{}) error

// InMemoryAguiProjection is an in-memory implementation of the AguiProjection.
//
// This projection is optimized for UI queries and rebuilt from the EventStore.
// It maintains:
//   - Active message streams (for streaming UI updates)
//   - Tool call execution tracking
//   - Agent activity metrics
//   - Run lifecycle state
//
// When an EventEmitter is registered, the projection emits AGUI protocol events
// in real-time as domain events are processed, enabling live UI updates.
type InMemoryAguiProjection struct {
	mu sync.RWMutex

	// Thread state indexed by threadID
	threads map[string]*AguiThreadState

	// Active runs indexed by threadID
	activeRuns map[string]*AguiRunState

	// Active messages being streamed (messageID -> message)
	streamingMessages map[string]*AguiMessage

	// Tool calls indexed by threadID
	toolCallsByThread map[string]map[string]*AguiToolCall

	// Event emitter for real-time AGUI events (optional)
	emitter EventEmitter
}

// NewInMemoryAguiProjection creates a new in-memory AGUI projection.
func NewInMemoryAguiProjection() *InMemoryAguiProjection {
	return &InMemoryAguiProjection{
		threads:           make(map[string]*AguiThreadState),
		activeRuns:        make(map[string]*AguiRunState),
		streamingMessages: make(map[string]*AguiMessage),
		toolCallsByThread: make(map[string]map[string]*AguiToolCall),
	}
}

// SetEventEmitter registers a callback for real-time AGUI event emission.
// The projection will call this emitter whenever it processes domain events,
// enabling real-time UI updates.
func (p *InMemoryAguiProjection) SetEventEmitter(emitter EventEmitter) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.emitter = emitter
}

// Apply updates the projection with a single event.
func (p *InMemoryAguiProjection) Apply(ctx context.Context, event core.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	threadID := event.AggregateID()
	if threadID == "" {
		return nil // Skip events without thread context
	}

	// Ensure thread state exists
	if _, exists := p.threads[threadID]; !exists {
		p.threads[threadID] = &AguiThreadState{
			ThreadID:      threadID,
			Messages:      []AguiMessage{},
			ToolCalls:     []AguiToolCall{},
			AgentActivity: make(map[string]*AguiAgentActivity),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		p.toolCallsByThread[threadID] = make(map[string]*AguiToolCall)
	}

	thread := p.threads[threadID]
	thread.UpdatedAt = time.Now()
	thread.Version++

	// Apply event-specific logic
	switch evt := event.(type) {
	case *core.ThreadCreatedEvent:
		thread.Title = evt.Title
		thread.CreatedAt = evt.Timestamp

	case *core.UserMessageAddedEvent:
		p.addMessage(thread, AguiMessage{
			ID:        evt.MessageID,
			Role:      "user",
			Content:   evt.Content,
			Timestamp: evt.Timestamp,
			Status:    "complete",
		})

	case *core.AssistantMessageStartedEvent:
		msg := AguiMessage{
			ID:        evt.MessageID,
			Role:      "assistant",
			Content:   "",
			Timestamp: evt.Timestamp,
			Status:    "streaming",
		}
		p.addMessage(thread, msg)
		p.streamingMessages[evt.MessageID] = &thread.Messages[len(thread.Messages)-1]

	case *core.AssistantContentStreamedEvent:
		if msg, exists := p.streamingMessages[evt.MessageID]; exists {
			msg.Content += evt.Delta
		}

	case *core.AssistantMessageCompletedEvent:
		if msg, exists := p.streamingMessages[evt.MessageID]; exists {
			msg.Status = "complete"
			msg.Content = evt.Content // Final content
			if len(evt.ToolCalls) > 0 {
				msg.ToolCalls = make([]AguiMessageToolCall, len(evt.ToolCalls))
				for i, tc := range evt.ToolCalls {
					msg.ToolCalls[i] = AguiMessageToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					}
				}
			}
			delete(p.streamingMessages, evt.MessageID)
		}
		p.updateAgentActivity(thread, evt.AgentName, evt.Timestamp)

	case *core.ToolCallStartedEvent:
		toolCall := &AguiToolCall{
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

	case *core.ToolCallArgumentsStreamedEvent:
		if tc, exists := p.toolCallsByThread[threadID][evt.ToolCallID]; exists {
			tc.Arguments += evt.Delta
		}

	case *core.ToolCallCompletedEvent:
		if tc, exists := p.toolCallsByThread[threadID][evt.ToolCallID]; exists {
			tc.Status = "args_complete"
		}

	case *core.ToolCallExecutedEvent:
		msg := AguiMessage{
			ID:        evt.MessageID,
			Role:      "tool",
			Content:   evt.ToolResult,
			AgentName: evt.AgentName,
			Timestamp: evt.Timestamp,
			Status:    "complete",
			ToolCall: &AguiMessageToolResult{
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

		// Emit TOOL_CALL_RESULT event for real-time UI updates
		toolResultEvent := aguievents.NewToolCallResultEvent(
			evt.MessageID,
			evt.ToolCallID,
			evt.ToolResult,
		)
		p.emit(threadID, toolResultEvent)

	case *core.ThreadTitleUpdatedEvent:
		thread.Title = evt.NewTitle

	case *core.ThreadDeletedEvent:
		delete(p.threads, threadID)
		delete(p.activeRuns, threadID)
		delete(p.toolCallsByThread, threadID)
	}

	return nil
}

// addMessage adds a message to the thread state.
func (p *InMemoryAguiProjection) addMessage(thread *AguiThreadState, msg AguiMessage) {
	thread.Messages = append(thread.Messages, msg)
}

// updateAgentActivity updates the agent activity metrics.
func (p *InMemoryAguiProjection) updateAgentActivity(thread *AguiThreadState, agentName string, timestamp time.Time) {
	if agentName == "" {
		return
	}

	activity, exists := thread.AgentActivity[agentName]
	if !exists {
		activity = &AguiAgentActivity{
			AgentName:     agentName,
			FirstActiveAt: timestamp,
		}
		thread.AgentActivity[agentName] = activity
	}

	activity.LastActiveAt = timestamp
}

// GetThreadState retrieves the UI state for a thread.
func (p *InMemoryAguiProjection) GetThreadState(threadID string) (*AguiThreadState, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modifications
	threadCopy := *thread
	threadCopy.Messages = make([]AguiMessage, len(thread.Messages))
	copy(threadCopy.Messages, thread.Messages)
	threadCopy.ToolCalls = make([]AguiToolCall, len(thread.ToolCalls))
	copy(threadCopy.ToolCalls, thread.ToolCalls)

	return &threadCopy, true
}

// GetActiveRun retrieves information about the currently active run for a thread.
func (p *InMemoryAguiProjection) GetActiveRun(threadID string) (*AguiRunState, bool) {
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
func (p *InMemoryAguiProjection) GetMessages(threadID string) []AguiMessage {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return []AguiMessage{}
	}

	messages := make([]AguiMessage, len(thread.Messages))
	copy(messages, thread.Messages)
	return messages
}

// GetToolCalls retrieves all tool calls for a thread.
func (p *InMemoryAguiProjection) GetToolCalls(threadID string) []AguiToolCall {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return []AguiToolCall{}
	}

	toolCalls := make([]AguiToolCall, len(thread.ToolCalls))
	copy(toolCalls, thread.ToolCalls)
	return toolCalls
}

// GetAgentActivity retrieves agent activity summary for a thread.
func (p *InMemoryAguiProjection) GetAgentActivity(threadID string) map[string]*AguiAgentActivity {
	p.mu.RLock()
	defer p.mu.RUnlock()

	thread, exists := p.threads[threadID]
	if !exists {
		return make(map[string]*AguiAgentActivity)
	}

	// Return a copy
	activity := make(map[string]*AguiAgentActivity, len(thread.AgentActivity))
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
func (p *InMemoryAguiProjection) Rebuild(ctx context.Context, store core.EventStore, threadID string) error {
	if threadID == "" {
		// Rebuild all streams - not supported with minimal interface
		return fmt.Errorf("RebuildAll not supported with minimal EventStore interface")
	}

	// Rebuild single stream
	p.mu.Lock()
	// Clear state for this stream
	delete(p.threads, threadID)
	delete(p.activeRuns, threadID)
	delete(p.toolCallsByThread, threadID)
	p.mu.Unlock()

	// Replay all events for the stream
	events, err := store.ReadStream(ctx, threadID, 0)
	if err != nil {
		return fmt.Errorf("failed to read events for stream %s: %w", threadID, err)
	}

	for _, event := range events {
		if err := p.Apply(ctx, event); err != nil {
			return fmt.Errorf("failed to apply event during rebuild: %w", err)
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

	p.threads = make(map[string]*AguiThreadState)
	p.activeRuns = make(map[string]*AguiRunState)
	p.streamingMessages = make(map[string]*AguiMessage)
	p.toolCallsByThread = make(map[string]map[string]*AguiToolCall)
}

// emit emits an AGUI event if an emitter is registered.
// This enables real-time UI updates without needing MESSAGES_SNAPSHOT.
func (p *InMemoryAguiProjection) emit(threadID string, event interface{}) {
	if p.emitter != nil {
		// The emitter is responsible for routing the event to the correct thread/job
		// We pass the threadID so the WorkerPool can route to the correct buffer
		go func() {
			_ = p.emitter(threadID, event)
		}()
	}
}

// Verify InMemoryAguiProjection implements AguiProjection
var _ AguiProjection = (*InMemoryAguiProjection)(nil)
