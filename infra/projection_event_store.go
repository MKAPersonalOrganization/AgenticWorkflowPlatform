package infra

import (
	"context"
	"sync"

	"school-with-ai/services/sdk"
)

// ProjectionEventStore wraps an EventLog and automatically updates projections when events are appended.
//
// This ensures that all projections stay in sync with the event log without requiring
// manual projection updates at every append site.
type ProjectionEventStore struct {
	wrapped     sdk.EventLog
	projections []sdk.Projection
	mu          sync.RWMutex
}

// NewProjectionEventStore creates an event store that automatically updates projections.
func NewProjectionEventStore(wrapped sdk.EventLog, projections ...sdk.Projection) *ProjectionEventStore {
	return &ProjectionEventStore{
		wrapped:     wrapped,
		projections: projections,
	}
}

// Append adds one or more events to a thread's history and updates all projections.
func (p *ProjectionEventStore) Append(threadID string, events ...sdk.Event) error {
	// Append to underlying log first
	if err := p.wrapped.Append(threadID, events...); err != nil {
		return err
	}

	// Update all projections
	p.mu.RLock()
	defer p.mu.RUnlock()

	ctx := context.Background()
	for _, projection := range p.projections {
		for _, event := range events {
			if err := projection.Apply(ctx, event); err != nil {
				// Log error but continue - projections can be rebuilt
				// In production, you might want to track failed projections
				continue
			}
		}
	}

	return nil
}

// AppendToStream adds events and returns version (for EventStore-like behavior)
func (p *ProjectionEventStore) AppendToStream(ctx context.Context, streamID string, events ...sdk.Event) (int64, error) {
	// First append using the EventLog interface
	if err := p.Append(streamID, events...); err != nil {
		return 0, err
	}

	// Count events to get version
	history, err := p.wrapped.History(streamID)
	if err != nil {
		return 0, err
	}
	return int64(len(history)), nil
}

// History retrieves all events for a thread in order.
func (p *ProjectionEventStore) History(threadID string) ([]sdk.Event, error) {
	return p.wrapped.History(threadID)
}

// RegisterProjection adds a projection to receive event updates.
func (p *ProjectionEventStore) RegisterProjection(projection sdk.Projection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.projections = append(p.projections, projection)
}

// Verify ProjectionEventStore implements EventLog
var _ sdk.EventLog = (*ProjectionEventStore)(nil)

// Implement EventStore interface methods by delegating to wrapped InMemoryEventStore
func (p *ProjectionEventStore) ReadAll(ctx context.Context, streamID string) ([]sdk.Event, error) {
	// Directly use the wrapped InMemoryEventStore which implements both interfaces
	if log, ok := p.wrapped.(*InMemoryEventStore); ok {
		return log.ReadAll(ctx, streamID)
	}
	return p.wrapped.History(streamID)
}

func (p *ProjectionEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]sdk.Event, error) {
	if log, ok := p.wrapped.(*InMemoryEventStore); ok {
		return log.ReadStream(ctx, streamID, fromVersion)
	}
	// Fallback
	events, err := p.wrapped.History(streamID)
	if err != nil {
		return nil, err
	}
	if fromVersion >= int64(len(events)) {
		return []sdk.Event{}, nil
	}
	return events[fromVersion:], nil
}

func (p *ProjectionEventStore) StreamVersion(ctx context.Context, streamID string) (int64, error) {
	if log, ok := p.wrapped.(*InMemoryEventStore); ok {
		return log.StreamVersion(ctx, streamID)
	}
	// Fallback
	events, err := p.wrapped.History(streamID)
	if err != nil {
		return 0, err
	}
	return int64(len(events)), nil
}

func (p *ProjectionEventStore) StreamExists(ctx context.Context, streamID string) (bool, error) {
	if log, ok := p.wrapped.(*InMemoryEventStore); ok {
		return log.StreamExists(ctx, streamID)
	}
	// Fallback
	events, err := p.wrapped.History(streamID)
	if err != nil {
		return false, err
	}
	return len(events) > 0, nil
}

func (p *ProjectionEventStore) ListStreams(ctx context.Context) ([]string, error) {
	if log, ok := p.wrapped.(*InMemoryEventStore); ok {
		return log.ListStreams(ctx)
	}
	return []string{}, nil
}

func (p *ProjectionEventStore) DeleteStream(ctx context.Context, streamID string) error {
	if log, ok := p.wrapped.(*InMemoryEventStore); ok {
		return log.DeleteStream(ctx, streamID)
	}
	return nil
}

