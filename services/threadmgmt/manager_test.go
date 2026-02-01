package threadmgmt

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
)

// ConcurrentMockEventStore simulates concurrency conflicts
type ConcurrentMockEventStore struct {
	mu            sync.Mutex
	events        map[string][]domain.Event
	versions      map[string]int64
	conflictCount int
	maxConflicts  int
}

func NewConcurrentMockEventStore(maxConflicts int) *ConcurrentMockEventStore {
	return &ConcurrentMockEventStore{
		events:       make(map[string][]domain.Event),
		versions:     make(map[string]int64),
		maxConflicts: maxConflicts,
	}
}

func (m *ConcurrentMockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...domain.Event) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentVersion := m.versions[streamID]

	// Simulate conflict
	if m.conflictCount < m.maxConflicts {
		m.conflictCount++
		// If we are at the first append or a specific one, we can simulate that someone else updated the stream
		// by just returning a conflict error.
		return 0, core.ErrConcurrencyConflict
	}

	if expectedVersion != -2 && expectedVersion != currentVersion {
		return 0, core.ErrConcurrencyConflict
	}

	m.events[streamID] = append(m.events[streamID], events...)
	m.versions[streamID] += int64(len(events))
	return m.versions[streamID], nil
}

func (m *ConcurrentMockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	events := m.events[streamID]
	if len(events) == 0 {
		return nil, domain.ErrThreadNotFound
	}
	return events, nil
}

func (m *ConcurrentMockEventStore) DeleteStream(ctx context.Context, streamID string) error {
	return nil
}

func TestManager_RetryUpdate(t *testing.T) {
	ctx := context.Background()
	log := logger.New(io.Discard, logger.LevelInfo, "test", nil)

	t.Run("Success after retries", func(t *testing.T) {
		// No conflicts for setup
		store := NewConcurrentMockEventStore(0)
		repo := domain.NewThreadRepository(store)
		mgr := New(log, repo)

		threadID := "test-thread-success"
		_, _, err := mgr.CreateThread(ctx, threadID, "Title")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// Reset conflict counter and set specific max conflicts for testing update
		store.mu.Lock()
		store.conflictCount = 0
		store.maxConflicts = 2
		store.mu.Unlock()

		_, _, err = mgr.AddToolResultMessage(ctx, threadID, "msg-123", "call-1", "tool-1", "result", "agent-1")
		if err != nil {
			t.Fatalf("Expected success after retries, got error: %v", err)
		}

		if store.conflictCount != 2 {
			t.Errorf("Expected 2 conflicts, got %d", store.conflictCount)
		}
	})

	t.Run("Failure after max retries", func(t *testing.T) {
		// No conflicts for setup
		store := NewConcurrentMockEventStore(0)
		repo := domain.NewThreadRepository(store)
		mgr := New(log, repo)

		threadID := "test-thread-fail"
		_, _, err := mgr.CreateThread(ctx, threadID, "Title")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		store.mu.Lock()
		store.conflictCount = 0
		store.maxConflicts = 11 // maxRetries is 10
		store.mu.Unlock()

		_, _, err = mgr.AddToolResultMessage(ctx, threadID, "msg-123", "call-1", "tool-1", "result", "agent-1")
		if err == nil {
			t.Fatal("Expected failure after max retries, got success")
		}

		if !errors.Is(err, core.ErrConcurrencyConflict) {
			t.Errorf("Expected concurrency conflict error, got: %v", err)
		}
	})
}
