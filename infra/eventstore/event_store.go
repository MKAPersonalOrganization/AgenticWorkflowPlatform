package eventstore

import (
	"context"
	"sync"

	"school-with-ai/sdk/agentsdk/core"
)

// InMemoryEventStore is an in-memory implementation of EventStore.
type InMemoryEventStore struct {
	mu      sync.RWMutex
	streams map[string][]core.Event
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		streams: make(map[string][]core.Event),
	}
}

// AppendWithVersion adds events to a stream with optimistic concurrency control.
func (s *InMemoryEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...core.Event) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	currentVersion := int64(len(s.streams[streamID]))

	// Check optimistic concurrency
	if expectedVersion >= 0 && currentVersion != expectedVersion {
		return 0, core.ErrConcurrencyConflict
	}

	s.streams[streamID] = append(s.streams[streamID], events...)

	return int64(len(s.streams[streamID])), nil
}

// ReadStream reads all events for a stream from a specific version.
func (s *InMemoryEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.streams[streamID]
	if fromVersion >= int64(len(events)) {
		return []core.Event{}, nil
	}

	return events[fromVersion:], nil
}

// DeleteStream removes a stream from the in-memory store.
func (s *InMemoryEventStore) DeleteStream(ctx context.Context, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.streams, streamID)

	return nil
}

// Verify InMemoryEventStore implements core.EventStore.
var _ core.EventStore = (*InMemoryEventStore)(nil)
