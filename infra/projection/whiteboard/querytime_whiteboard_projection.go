package whiteboard

import (
	"context"
	"encoding/json"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// QueryTimeWhiteboardProjection is a query-time projection that rebuilds whiteboard state
// from events on every read. This eliminates the need for in-memory state management.
//
// Benefits:
//   - No memory overhead for storing whiteboard state
//   - Always consistent with event store (no projection lag)
//   - Simpler architecture - no need to manage projection updates
//   - Thread state persists across server restarts
type QueryTimeWhiteboardProjection struct {
	eventStore core.EventStore
}

// NewQueryTimeWhiteboardProjection creates a new query-time whiteboard projection.
func NewQueryTimeWhiteboardProjection(eventStore core.EventStore) *QueryTimeWhiteboardProjection {
	return &QueryTimeWhiteboardProjection{
		eventStore: eventStore,
	}
}

// Get retrieves the whiteboard content for a thread by rebuilding from events.
func (p *QueryTimeWhiteboardProjection) Get(threadID string) string {
	ctx := context.Background()

	// Use same stream naming convention as thread aggregate: "Thread-{threadID}"
	streamName := "Thread-" + threadID
	events, err := p.eventStore.ReadStream(ctx, streamName, 0)
	if err != nil || len(events) == 0 {
		return ""
	}

	// Rebuild whiteboard state from events
	content := ""
	for _, event := range events {
		content = applyEventToWhiteboard(content, event)
	}

	return content
}

// Update is a no-op for query-time projections since we read directly from the event store.
func (p *QueryTimeWhiteboardProjection) Update(event any) error {
	// No-op: query-time projections don't maintain state
	return nil
}

// Apply is a no-op for query-time projections.
func (p *QueryTimeWhiteboardProjection) Apply(ctx context.Context, event core.Event) error {
	// No-op: query-time projections don't maintain state
	return nil
}

// Rebuild is a no-op for query-time projections since we always read fresh from the event store.
func (p *QueryTimeWhiteboardProjection) Rebuild(ctx context.Context, store core.EventStore, streamID string) error {
	// No-op: query-time projections don't maintain state, so nothing to rebuild
	return nil
}

// applyEventToWhiteboard applies an event to rebuild whiteboard state.
func applyEventToWhiteboard(currentContent string, event core.Event) string {
	// Rebuild from MessageAdded events with toolCalls
	switch evt := event.(type) {
	case *domain.MessageAdded:
		// Check if assistant message has write_whiteboard tool call
		for _, tc := range evt.ToolCalls {
			if tc.Function.Name == "write_whiteboard" {
				return applyWhiteboardTool(currentContent, tc.Function.Arguments)
			}
		}
	case domain.MessageAdded:
		// Check if assistant message has write_whiteboard tool call
		for _, tc := range evt.ToolCalls {
			if tc.Function.Name == "write_whiteboard" {
				return applyWhiteboardTool(currentContent, tc.Function.Arguments)
			}
		}
	}

	return currentContent
}

// applyWhiteboardTool parses whiteboard tool arguments and applies the action.
func applyWhiteboardTool(currentContent string, toolArguments string) string {
	var args struct {
		Content string `json:"content"`
		Action  string `json:"action"`
	}

	err := json.Unmarshal([]byte(toolArguments), &args)
	if err != nil {
		return currentContent // Skip malformed events
	}

	switch args.Action {
	case "write":
		return args.Content
	case "append":
		if currentContent == "" {
			return args.Content
		}

		return currentContent + "\n" + args.Content
	case "clear":
		return ""
	default:
		return currentContent
	}
}

// Verify QueryTimeWhiteboardProjection implements WhiteboardStore.
var _ WhiteboardStore = (*QueryTimeWhiteboardProjection)(nil)
