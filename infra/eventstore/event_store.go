package eventstore

import (
	"context"
	"fmt"
	"log/slog"
	"school-with-ai/sdk/agentsdk/core"
	"sync"
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
		slog.Debug("Concurrency conflict in event store",
			"stream", streamID,
			"expected", expectedVersion,
			"current", currentVersion)
		return 0, core.ErrConcurrencyConflict
	}

	slog.Debug("AppendWithVersion to event store",
		"stream", streamID,
		"expected", expectedVersion,
		"current", currentVersion,
		"eventCount", len(events))

	for i, evt := range events {
		slog.Debug("Appending event to stream",
			"stream", streamID,
			"index", i,
			"eventType", fmt.Sprintf("%T", evt))
	}

	s.streams[streamID] = append(s.streams[streamID], events...)

	newVersion := int64(len(s.streams[streamID]))
	slog.Debug("Stream updated in event store",
		"stream", streamID,
		"newVersion", newVersion)

	// Verify what's actually in the stream
	for i, evt := range s.streams[streamID] {
		slog.Debug("Stream event",
			"stream", streamID,
			"index", i,
			"eventType", fmt.Sprintf("%T", evt))
	}

	return newVersion, nil
}

// ReadStream reads all events for a stream from a specific version.
func (s *InMemoryEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.streams[streamID]
	if fromVersion >= int64(len(events)) {
		return []core.Event{}, nil
	}

	result := events[fromVersion:]
	slog.Debug("ReadStream from event store",
		"stream", streamID,
		"fromVersion", fromVersion,
		"eventCount", len(result))
	for i, evt := range result {
		slog.Debug("Returning event from stream",
			"stream", streamID,
			"index", i,
			"eventType", fmt.Sprintf("%T", evt))
	}

	return result, nil
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
