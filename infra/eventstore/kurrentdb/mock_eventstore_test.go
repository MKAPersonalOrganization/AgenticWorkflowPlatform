package kurrentdb

import (
	"context"
	"fmt"
	"sync"

	"school-with-ai/sdk/agentsdk/core"
)

// MockEventStore is a test double for EventStore that doesn't require KurrentDB
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

func (m *MockEventStore) Append(ctx context.Context, streamID string, events ...core.Event) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleted[streamID] {
		return 0, fmt.Errorf("stream %s has been deleted", streamID)
	}

	if m.streams[streamID] == nil {
		m.streams[streamID] = make([]core.Event, 0)
	}

	m.streams[streamID] = append(m.streams[streamID], events...)
	return int64(len(m.streams[streamID])), nil
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

// ReadAllThreadIDs returns all stream IDs (for testing)
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

// Verify MockEventStore implements EventStore
var _ core.EventStore = (*MockEventStore)(nil)
