package infra

import (
	"context"
	"sync"

	"school-with-ai/services/sdk"
)

// InMemoryEventStore is an in-memory implementation of EventLog and EventStore
type InMemoryEventStore struct {
	mu     sync.RWMutex
	events map[string][]sdk.Event
}

// NewInMemoryEventStore creates a new in-memory event store
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events: make(map[string][]sdk.Event),
	}
}

// Append adds one or more events to a thread's history (EventLog interface)
func (s *InMemoryEventStore) Append(threadID string, events ...sdk.Event) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events[threadID] = append(s.events[threadID], events...)
	return nil
}

// History retrieves all events for a thread in order
func (s *InMemoryEventStore) History(threadID string) ([]sdk.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.events[threadID], nil
}

// ReadAll retrieves all events for a stream (implements EventStore)
func (s *InMemoryEventStore) ReadAll(ctx context.Context, streamID string) ([]sdk.Event, error) {
	return s.History(streamID)
}

// ReadStream reads events from a specific version (implements EventStore)
func (s *InMemoryEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]sdk.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.events[streamID]
	if fromVersion >= int64(len(events)) {
		return []sdk.Event{}, nil
	}
	return events[fromVersion:], nil
}

// StreamVersion returns the current version for a stream (implements EventStore)
func (s *InMemoryEventStore) StreamVersion(ctx context.Context, streamID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.events[streamID])), nil
}

// StreamExists checks if a stream has any events (implements EventStore)
func (s *InMemoryEventStore) StreamExists(ctx context.Context, streamID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.events[streamID]
	return exists, nil
}

// ListStreams returns all stream IDs (implements EventStore)
func (s *InMemoryEventStore) ListStreams(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	streams := make([]string, 0, len(s.events))
	for streamID := range s.events {
		streams = append(streams, streamID)
	}
	return streams, nil
}

// DeleteStream removes all events for a stream (implements EventStore)
func (s *InMemoryEventStore) DeleteStream(ctx context.Context, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.events, streamID)
	return nil
}

// Verify InMemoryEventStore implements EventLog
var _ sdk.EventLog = (*InMemoryEventStore)(nil)
