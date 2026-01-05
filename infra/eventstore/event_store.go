package eventstore

import (
	"context"
	"sync"

	sdk "school-with-ai/sdk/agent-sdk/core"
)

// InMemoryEventStore is an in-memory implementation of EventStore.
type InMemoryEventStore struct {
	mu      sync.RWMutex
	streams map[string][]sdk.Event
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		streams: make(map[string][]sdk.Event),
	}
}

// Append adds events to a stream.
func (s *InMemoryEventStore) Append(ctx context.Context, streamID string, events ...sdk.Event) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.streams[streamID] = append(s.streams[streamID], events...)
	return int64(len(s.streams[streamID])), nil
}

// ReadStream reads all events for a stream from a specific version.
func (s *InMemoryEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]sdk.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.streams[streamID]
	if fromVersion >= int64(len(events)) {
		return []sdk.Event{}, nil
	}

	return events[fromVersion:], nil
}

// Verify InMemoryEventStore implements sdk.EventStore
var _ sdk.EventStore = (*InMemoryEventStore)(nil)
