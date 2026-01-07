package whiteboard

import (
	"context"
	"encoding/json"
	"sync"

	"school-with-ai/sdk/agentsdk/core"
)

// WhiteboardStore is a projection that maintains whiteboard content per thread.
//
// This projection listens to ToolCallExecutedEvent (for write_whiteboard tool) and builds
// a read model of whiteboard state. It's used by tools to read/write whiteboard
// content without coupling to the Thread domain model.
//
// Event sourced: All state is derived from events in the EventStore.
// Thread-safe: All operations are protected by mutex.
type WhiteboardStore interface {
	// Get retrieves the whiteboard content for a thread.
	// Returns empty string if thread has no whiteboard content.
	Get(threadID string) string

	// GetAll retrieves all whiteboards (threadID -> content).
	GetAll() map[string]string

	// Update applies an event to update the projection.
	// Implements eventstore.EventApplier interface.
	Update(event any) error
}

// InMemoryWhiteboardProjection is an in-memory implementation of WhiteboardStore.
type InMemoryWhiteboardProjection struct {
	mu          sync.RWMutex
	whiteboards map[string]string // threadID -> whiteboard content
}

// NewInMemoryWhiteboardProjection creates a new in-memory whiteboard projection.
func NewInMemoryWhiteboardProjection() *InMemoryWhiteboardProjection {
	return &InMemoryWhiteboardProjection{
		whiteboards: make(map[string]string),
	}
}

// Get retrieves the whiteboard content for a thread.
func (p *InMemoryWhiteboardProjection) Get(threadID string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.whiteboards[threadID]
}

// GetAll retrieves all whiteboards.
func (p *InMemoryWhiteboardProjection) GetAll() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]string, len(p.whiteboards))
	for k, v := range p.whiteboards {
		result[k] = v
	}
	return result
}

// Update applies an event to update the projection.
func (p *InMemoryWhiteboardProjection) Update(event any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Handle standard ToolCallExecutedEvent for write_whiteboard tool
	if toolEvent, ok := event.(*core.ToolCallExecutedEvent); ok {
		if toolEvent.ToolName == "write_whiteboard" {
			// Parse tool arguments to extract action and content
			var args struct {
				Content string `json:"content"`
				Action  string `json:"action"`
			}
			if err := json.Unmarshal([]byte(toolEvent.ToolArguments), &args); err != nil {
				return nil // Skip malformed events
			}

			threadID := toolEvent.AggregateID()
			action := args.Action
			content := args.Content

			switch action {
			case "write":
				p.whiteboards[threadID] = content
			case "append":
				existing := p.whiteboards[threadID]
				if existing == "" {
					p.whiteboards[threadID] = content
				} else {
					p.whiteboards[threadID] = existing + "\n" + content
				}
			case "clear":
				p.whiteboards[threadID] = ""
			}
		}
		return nil
	}

	// Ignore other events
	return nil
}

// Apply updates the projection with a single event (implements eventstore.Projection).
func (p *InMemoryWhiteboardProjection) Apply(ctx context.Context, event core.Event) error {
	return p.Update(event)
}

// Rebuild reconstructs the projection from the EventStore (implements eventstore.Projection).
func (p *InMemoryWhiteboardProjection) Rebuild(ctx context.Context, store core.EventStore, streamID string) error {
	// Read all events for the stream
	events, err := store.ReadStream(ctx, streamID, 0)
	if err != nil {
		return err
	}

	// Apply each event
	for _, event := range events {
		if err := p.Apply(ctx, event); err != nil {
			return err
		}
	}

	return nil
}
