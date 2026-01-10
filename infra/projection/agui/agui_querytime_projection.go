package agui

import (
	"context"
	"fmt"
	"log/slog"
	"school-with-ai/domain"
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

	// Read all events for this thread (using aggregate stream naming)
	streamName := fmt.Sprintf("Thread-%s", threadID)
	events, err := p.eventStore.ReadStream(ctx, streamName, 0)
	if err != nil || len(events) == 0 {
		slog.Debug("GetThreadState: No events for thread",
			"threadID", threadID,
			"error", err)
		return nil, false
	}

	slog.Debug("GetThreadState: Reading events for thread",
		"threadID", threadID,
		"eventCount", len(events))

	state := &AguiThreadState{
		ThreadID:      threadID,
		Messages:      []aguievents.Message{},
		ToolCalls:     []AguiToolCall{},
		AgentActivity: make(map[string]*AguiAgentActivity),
		CreatedAt:     time.Time{}, // Will be set from ThreadCreated event
		UpdatedAt:     time.Time{}, // Will be updated as events are applied
	}

	streamingMessages := make(map[string]*aguievents.Message)
	toolCallsByID := make(map[string]*AguiToolCall)
	var activeRun *AguiRunState

	for i, event := range events {
		slog.Debug("Applying event to thread state",
			"threadID", threadID,
			"eventIndex", i,
			"eventType", event.EventType())
		p.applyEventToState(event, state, streamingMessages, toolCallsByID, &activeRun)
	}

	slog.Debug("GetThreadState: Final state",
		"threadID", threadID,
		"title", state.Title,
		"messageCount", len(state.Messages),
		"version", state.Version,
		"isDeleted", state.IsDeleted)

	// Don't return deleted threads
	if state.IsDeleted {
		return nil, false
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
		slog.Debug("ListThreads: threadIDLister is nil")
		return []string{}
	}

	ctx := context.Background()
	threadIDs, err := p.threadIDLister.ReadAllThreadIDs(ctx)
	if err != nil {
		slog.Debug("ListThreads: Error reading thread IDs", "error", err)
		return []string{}
	}
	slog.Debug("ListThreads returning threads", "count", len(threadIDs))

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
	// Domain events (from event store - source of truth)
	case *domain.ThreadCreated:
		state.Title = evt.Title
		state.CreatedAt = evt.Timestamp
	case domain.ThreadCreated:
		state.Title = evt.Title
		state.CreatedAt = evt.Timestamp

	case *domain.MessageAdded:
		// Check for duplicates before adding
		if !messageExists(state.Messages, evt.MessageID) {
			content := evt.Content
			msg := aguievents.Message{
				ID:      evt.MessageID,
				Role:    evt.Role,
				Content: &content,
			}
			state.Messages = append(state.Messages, msg)
		}
	case domain.MessageAdded:
		// Check for duplicates before adding
		if !messageExists(state.Messages, evt.MessageID) {
			content := evt.Content
			msg := aguievents.Message{
				ID:      evt.MessageID,
				Role:    evt.Role,
				Content: &content,
			}
			state.Messages = append(state.Messages, msg)
		}

	case *domain.ThreadTitleChanged:
		state.Title = evt.NewTitle
	case domain.ThreadTitleChanged:
		state.Title = evt.NewTitle

	case *domain.ThreadDeleted:
		// Mark as deleted
		state.IsDeleted = true
	case domain.ThreadDeleted:
		// Mark as deleted
		state.IsDeleted = true
	}
}

// messageExists checks if a message with the given ID already exists in the messages slice.
// This prevents duplicate messages during the transition from UI events to domain events.
func messageExists(messages []aguievents.Message, messageID string) bool {
	for _, msg := range messages {
		if msg.ID == messageID {
			return true
		}
	}
	return false
}

// SetEventEmitter is a no-op for query-time projections.
// Real-time event emission is not supported since we don't maintain state.
func (p *QueryTimeAguiProjection) SetEventEmitter(emitter EventEmitter) {
	// No-op: query-time projections don't emit real-time events
}

// Verify QueryTimeAguiProjection implements AguiProjection.
var _ AguiProjection = (*QueryTimeAguiProjection)(nil)
