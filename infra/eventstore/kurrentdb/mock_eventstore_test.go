package kurrentdb

import (
	"context"
	"errors"
	"sync"

	"school-with-ai/sdk/agentsdk/core"
)

// ErrStreamDeleted is returned when attempting to append to a deleted stream.
var ErrStreamDeleted = errors.New("stream has been deleted")

// MockEventStore is a test double for EventStore that doesn't require KurrentDB.
type MockEventStore struct {
	mu      sync.RWMutex
	streams map[string][]core.Event
	deleted map[string]bool
}

func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		streams: make(map[string][]core.Event),
		deleted: make(map[string]bool),
	}
}

func (m *MockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.deleted[streamID] {
		return []core.Event{}, nil
	}

	stream, exists := m.streams[streamID]
	if !exists {
		return []core.Event{}, nil
	}

	if fromVersion < 0 {
		fromVersion = 0
	}

	if fromVersion >= int64(len(stream)) {
		return []core.Event{}, nil
	}

	return stream[fromVersion:], nil
}

func (m *MockEventStore) DeleteStream(ctx context.Context, streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleted[streamID] = true
	delete(m.streams, streamID)

	return nil
}

// ReadAllThreadIDs returns all stream IDs (for testing).
func (m *MockEventStore) ReadAllThreadIDs(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.streams))
	for id := range m.streams {
		if !m.deleted[id] {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// AppendWithVersion adds events to a stream with optimistic concurrency control.
func (m *MockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...core.Event) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleted[streamID] {
		return 0, errors.New("stream is deleted")
	}

	// Check expected version if specified
	if expectedVersion >= 0 {
		currentVersion := int64(len(m.streams[streamID])) - 1
		if currentVersion != expectedVersion {
			return 0, core.ErrConcurrencyConflict
		}
	}

	// Append events
	m.streams[streamID] = append(m.streams[streamID], events...)
	return int64(len(m.streams[streamID])), nil
}

// Verify MockEventStore implements EventStore.
var _ core.EventStore = (*MockEventStore)(nil)
