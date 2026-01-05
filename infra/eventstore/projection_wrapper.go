package eventstore

import (
	"context"
	"sync"

	sdk "school-with-ai/sdk/agent-sdk/core"
)

// ProjectionEventStore wraps an EventStore and automatically updates projections when events are appended.
//
// This ensures that all projections stay in sync with the event store without requiring
// manual projection updates at every append site.
type ProjectionEventStore struct {
	wrapped     sdk.EventStore
	projections []sdk.Projection
	mu          sync.RWMutex
}

// NewProjectionEventStore creates an event store that automatically updates projections.
func NewProjectionEventStore(wrapped sdk.EventStore, projections ...sdk.Projection) *ProjectionEventStore {
	return &ProjectionEventStore{
		wrapped:     wrapped,
		projections: projections,
	}
}

// Append adds one or more events to a stream and updates all projections.
// Implements EventStore interface.
func (p *ProjectionEventStore) Append(ctx context.Context, streamID string, events ...sdk.Event) (int64, error) {
	// Append to underlying store first
	version, err := p.wrapped.Append(ctx, streamID, events...)
	if err != nil {
		return 0, err
	}

	// Update all projections
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, projection := range p.projections {
		for _, event := range events {
			if err := projection.Apply(ctx, event); err != nil {
				// Log error but continue - projections can be rebuilt
				// In production, you might want to track failed projections
				continue
			}
		}
	}

	return version, nil
}

// RegisterProjection adds a projection to receive event updates.
func (p *ProjectionEventStore) RegisterProjection(projection sdk.Projection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.projections = append(p.projections, projection)
}

// Verify ProjectionEventStore implements EventStore
var _ sdk.EventStore = (*ProjectionEventStore)(nil)

// Implement EventStore interface methods by delegating to wrapped EventStore
func (p *ProjectionEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]sdk.Event, error) {
	return p.wrapped.ReadStream(ctx, streamID, fromVersion)
}
