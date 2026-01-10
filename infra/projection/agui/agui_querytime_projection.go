package agui

import (
	"context"
	"fmt"
	"school-with-ai/sdk/agentsdk/core"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// QueryTimeAguiProjection is a query-time implementation of AguiProjection.
// Instead of maintaining in-memory state, it reads events from the event store
// on every query and rebuilds the projection state on-demand.
//
// This eliminates memory overhead and state loss on restart, but may be slower
// for queries. Use ReadAllThreadIDs to discover all threads.
type QueryTimeAguiProjection struct {
	eventStore     core.EventStore
	threadIDLister ThreadIDLister
}

// ThreadIDLister is an interface for event stores that can list all thread IDs.
type ThreadIDLister interface {
	ReadAllThreadIDs(ctx context.Context) ([]string, error)
}

// NewQueryTimeAguiProjection creates a new query-time AGUI projection.
func NewQueryTimeAguiProjection(eventStore core.EventStore) *QueryTimeAguiProjection {
	// Check if event store supports listing thread IDs
	lister, _ := eventStore.(ThreadIDLister)

	return &QueryTimeAguiProjection{
		eventStore:     eventStore,
		threadIDLister: lister,
	}
}

// GetThreadState rebuilds and returns the UI state for a thread from events.
func (p *QueryTimeAguiProjection) GetThreadState(threadID string) (*AguiThreadState, bool) {
	ctx := context.Background()
	events, err := p.eventStore.ReadStream(ctx, threadID, 0)
	if err != nil || len(events) == 0 {
		return nil, false
	}

	state := &AguiThreadState{
		ThreadID:      threadID,
		Messages:      []aguievents.Message{},
		ToolCalls:     []AguiToolCall{},
		AgentActivity: make(map[string]*AguiAgentActivity),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	streamingMessages := make(map[string]*aguievents.Message)
	toolCallsByID := make(map[string]*AguiToolCall)
	var activeRun *AguiRunState

	for _, event := range events {
		p.applyEventToState(event, state, streamingMessages, toolCallsByID, &activeRun)
	}

	state.ActiveRun = activeRun

	return state, true
}

// GetActiveRun retrieves the currently active run for a thread.
func (p *QueryTimeAguiProjection) GetActiveRun(threadID string) (*AguiRunState, bool) {
	state, exists := p.GetThreadState(threadID)
	if !exists || state.ActiveRun == nil {
		return nil, false
	}

	return state.ActiveRun, true
}

// GetMessages retrieves all UI-formatted messages for a thread.
func (p *QueryTimeAguiProjection) GetMessages(threadID string) []aguievents.Message {
	state, exists := p.GetThreadState(threadID)
	if !exists {
		return []aguievents.Message{}
	}

	return state.Messages
}

// ListThreads returns all thread IDs.
// Requires the event store to implement ThreadIDLister.
func (p *QueryTimeAguiProjection) ListThreads() []string {
	if p.threadIDLister == nil {
		fmt.Println("[AGUI QueryTime] threadIDLister is nil")

		return []string{}
	}

	ctx := context.Background()
	threadIDs, err := p.threadIDLister.ReadAllThreadIDs(ctx)
	if err != nil {
		fmt.Printf("[AGUI QueryTime] Error reading thread IDs: %v\n", err)

		return []string{}
	}
	fmt.Printf("[AGUI QueryTime] ListThreads returning %d threads\n", len(threadIDs))

	return threadIDs
}

// Apply is a no-op for query-time projections.
// State is rebuilt on every query, not maintained.
func (p *QueryTimeAguiProjection) Apply(ctx context.Context, event core.Event) error {
	return nil
}

// Update is a no-op for query-time projections.
func (p *QueryTimeAguiProjection) Update(ctx context.Context, events []core.Event) error {
	return nil
}

// ThreadExists checks if a thread exists by attempting to read its events.
func (p *QueryTimeAguiProjection) ThreadExists(threadID string) bool {
	_, exists := p.GetThreadState(threadID)

	return exists
}

// Rebuild is a no-op for query-time projections.
func (p *QueryTimeAguiProjection) Rebuild(ctx context.Context, store core.EventStore, threadID string) error {
	return nil
}

// applyEventToState applies a single event to the thread state during rebuild.
func (p *QueryTimeAguiProjection) applyEventToState(
	event core.Event,
	state *AguiThreadState,
	streamingMessages map[string]*aguievents.Message,
	toolCallsByID map[string]*AguiToolCall,
	activeRun **AguiRunState,
) {
	state.UpdatedAt = event.EventTimestamp()
	state.Version++

	switch evt := event.(type) {
	// Handle both pointer and value types for each event
	case *core.ThreadCreatedEvent:
		state.Title = evt.Title
		state.CreatedAt = evt.Timestamp
	case core.ThreadCreatedEvent:
		state.Title = evt.Title
		state.CreatedAt = evt.Timestamp

	case *core.UserMessageAddedEvent:
		content := evt.Content
		msg := aguievents.Message{
			ID:      evt.MessageID,
			Role:    "user",
			Content: &content,
		}
		state.Messages = append(state.Messages, msg)
	case core.UserMessageAddedEvent:
		content := evt.Content
		msg := aguievents.Message{
			ID:      evt.MessageID,
			Role:    "user",
			Content: &content,
		}
		state.Messages = append(state.Messages, msg)

	case *core.AssistantMessageStartedEvent:
		emptyContent := ""
		streamingMessages[evt.MessageID] = &aguievents.Message{
			ID:      evt.MessageID,
			Role:    "assistant",
			Content: &emptyContent,
		}
	case core.AssistantMessageStartedEvent:
		emptyContent := ""
		streamingMessages[evt.MessageID] = &aguievents.Message{
			ID:      evt.MessageID,
			Role:    "assistant",
			Content: &emptyContent,
		}

	case *core.AssistantContentStreamedEvent:
		if msg, exists := streamingMessages[evt.MessageID]; exists {
			if msg.Content != nil {
				updatedContent := *msg.Content + evt.Delta
				msg.Content = &updatedContent
			}
		}
	case core.AssistantContentStreamedEvent:
		if msg, exists := streamingMessages[evt.MessageID]; exists {
			if msg.Content != nil {
				updatedContent := *msg.Content + evt.Delta
				msg.Content = &updatedContent
			}
		}

	case *core.ToolCallStartedEvent:
		toolCall := &AguiToolCall{
			ID:              evt.ToolCallID,
			Name:            evt.ToolCallName,
			Arguments:       "",
			ParentMessageID: evt.ParentMessageID,
			AgentName:       evt.AgentName,
			StartedAt:       evt.Timestamp,
		}
		toolCallsByID[evt.ToolCallID] = toolCall
		if msg, exists := streamingMessages[evt.ParentMessageID]; exists {
			if msg.ToolCalls == nil {
				msg.ToolCalls = []aguievents.ToolCall{}
			}
			msg.ToolCalls = append(msg.ToolCalls, aguievents.ToolCall{
				ID:   evt.ToolCallID,
				Type: "function",
				Function: aguievents.Function{
					Name:      evt.ToolCallName,
					Arguments: "",
				},
			})
		}
	case core.ToolCallStartedEvent:
		toolCall := &AguiToolCall{
			ID:              evt.ToolCallID,
			Name:            evt.ToolCallName,
			Arguments:       "",
			ParentMessageID: evt.ParentMessageID,
			AgentName:       evt.AgentName,
			StartedAt:       evt.Timestamp,
		}
		toolCallsByID[evt.ToolCallID] = toolCall
		if msg, exists := streamingMessages[evt.ParentMessageID]; exists {
			if msg.ToolCalls == nil {
				msg.ToolCalls = []aguievents.ToolCall{}
			}
			msg.ToolCalls = append(msg.ToolCalls, aguievents.ToolCall{
				ID:   evt.ToolCallID,
				Type: "function",
				Function: aguievents.Function{
					Name:      evt.ToolCallName,
					Arguments: "",
				},
			})
		}

	case *core.ToolCallArgumentsStreamedEvent:
		if toolCall, exists := toolCallsByID[evt.ToolCallID]; exists {
			toolCall.Arguments += evt.Delta
		}
		// Update in streaming message as well
		for _, msg := range streamingMessages {
			for i := range msg.ToolCalls {
				if msg.ToolCalls[i].ID == evt.ToolCallID {
					msg.ToolCalls[i].Function.Arguments += evt.Delta
				}
			}
		}
	case core.ToolCallArgumentsStreamedEvent:
		if toolCall, exists := toolCallsByID[evt.ToolCallID]; exists {
			toolCall.Arguments += evt.Delta
		}
		for _, msg := range streamingMessages {
			for i := range msg.ToolCalls {
				if msg.ToolCalls[i].ID == evt.ToolCallID {
					msg.ToolCalls[i].Function.Arguments += evt.Delta
				}
			}
		}

	case *core.ToolCallCompletedEvent:
		// Mark tool call as having complete arguments
	case core.ToolCallCompletedEvent:
		// Mark tool call as having complete arguments

	case *core.AssistantMessageCompletedEvent:
		if msg, exists := streamingMessages[evt.MessageID]; exists {
			msg.Content = &evt.Content
			if len(evt.ToolCalls) > 0 {
				msg.ToolCalls = make([]aguievents.ToolCall, len(evt.ToolCalls))
				for i, tc := range evt.ToolCalls {
					msg.ToolCalls[i] = aguievents.ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: aguievents.Function{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
			}
			if evt.AgentName != "" {
				msg.Name = &evt.AgentName
			}
			state.Messages = append(state.Messages, *msg)
			delete(streamingMessages, evt.MessageID)
		}

		// Track agent activity
		if evt.AgentName != "" {
			if _, exists := state.AgentActivity[evt.AgentName]; !exists {
				state.AgentActivity[evt.AgentName] = &AguiAgentActivity{
					AgentName:     evt.AgentName,
					MessageCount:  0,
					LastActiveAt:  evt.Timestamp,
					FirstActiveAt: evt.Timestamp,
				}
			}
			state.AgentActivity[evt.AgentName].MessageCount++
			state.AgentActivity[evt.AgentName].LastActiveAt = evt.Timestamp
		}
	case core.AssistantMessageCompletedEvent:
		if msg, exists := streamingMessages[evt.MessageID]; exists {
			msg.Content = &evt.Content
			if len(evt.ToolCalls) > 0 {
				msg.ToolCalls = make([]aguievents.ToolCall, len(evt.ToolCalls))
				for i, tc := range evt.ToolCalls {
					msg.ToolCalls[i] = aguievents.ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: aguievents.Function{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
			}
			if evt.AgentName != "" {
				msg.Name = &evt.AgentName
			}
			state.Messages = append(state.Messages, *msg)
			delete(streamingMessages, evt.MessageID)
		}

		if evt.AgentName != "" {
			if _, exists := state.AgentActivity[evt.AgentName]; !exists {
				state.AgentActivity[evt.AgentName] = &AguiAgentActivity{
					AgentName:     evt.AgentName,
					MessageCount:  0,
					LastActiveAt:  evt.Timestamp,
					FirstActiveAt: evt.Timestamp,
				}
			}
			state.AgentActivity[evt.AgentName].MessageCount++
			state.AgentActivity[evt.AgentName].LastActiveAt = evt.Timestamp
		}

	case *core.ToolCallExecutedEvent:
		toolResult := evt.ToolResult
		toolCallID := evt.ToolCallID
		msg := aguievents.Message{
			ID:         evt.MessageID,
			Role:       "tool",
			Content:    &toolResult,
			ToolCallID: &toolCallID,
		}
		state.Messages = append(state.Messages, msg)

		if toolCall, exists := toolCallsByID[evt.ToolCallID]; exists {
			toolCall.Result = evt.ToolResult
			toolCall.CompletedAt = evt.Timestamp
			state.ToolCalls = append(state.ToolCalls, *toolCall)
		}
	case core.ToolCallExecutedEvent:
		toolResult := evt.ToolResult
		toolCallID := evt.ToolCallID
		msg := aguievents.Message{
			ID:         evt.MessageID,
			Role:       "tool",
			Content:    &toolResult,
			ToolCallID: &toolCallID,
		}
		state.Messages = append(state.Messages, msg)

		if toolCall, exists := toolCallsByID[evt.ToolCallID]; exists {
			toolCall.Result = evt.ToolResult
			toolCall.CompletedAt = evt.Timestamp
			state.ToolCalls = append(state.ToolCalls, *toolCall)
		}

	case *core.ThreadTitleUpdatedEvent:
		state.Title = evt.NewTitle
	case core.ThreadTitleUpdatedEvent:
		state.Title = evt.NewTitle

	case *core.ThreadDeletedEvent:
		// Mark as deleted (could add a Deleted flag to state if needed)
	case core.ThreadDeletedEvent:
		// Mark as deleted
	}
}

// SetEventEmitter is a no-op for query-time projections.
// Real-time event emission is not supported since we don't maintain state.
func (p *QueryTimeAguiProjection) SetEventEmitter(emitter EventEmitter) {
	// No-op: query-time projections don't emit real-time events
}

// Verify QueryTimeAguiProjection implements AguiProjection.
var _ AguiProjection = (*QueryTimeAguiProjection)(nil)
