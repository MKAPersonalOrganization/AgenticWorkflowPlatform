package threadmgmt

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"aicompass/foundation/logger"
	threadpkg "aicompass/modules/chat/thread"
)

// ConcurrentMockEventStore simulates concurrency conflicts
type ConcurrentMockEventStore struct {
	mu            sync.Mutex
	events        map[string][]threadpkg.Event
	versions      map[string]int64
	conflictCount int
	maxConflicts  int
}

func NewConcurrentMockEventStore(maxConflicts int) *ConcurrentMockEventStore {
	return &ConcurrentMockEventStore{
		events:       make(map[string][]threadpkg.Event),
		versions:     make(map[string]int64),
		maxConflicts: maxConflicts,
	}
}

func (m *ConcurrentMockEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...threadpkg.Event) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentVersion := m.versions[streamID]

	// Simulate conflict
	if m.conflictCount < m.maxConflicts {
		m.conflictCount++
		return 0, threadpkg.ErrConcurrencyConflict
	}

	if expectedVersion != -2 && expectedVersion != currentVersion {
		return 0, threadpkg.ErrConcurrencyConflict
	}

	m.events[streamID] = append(m.events[streamID], events...)
	m.versions[streamID] += int64(len(events))
	return m.versions[streamID], nil
}

func (m *ConcurrentMockEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]threadpkg.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	events, ok := m.events[streamID]
	if !ok {
		return nil, threadpkg.ErrThreadNotFound
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
		repo := threadpkg.NewThreadRepository(store)
		mgr := New(log, repo)

		threadID := "test-thread-success"
		_, _, err := mgr.CreateThread(ctx, threadID, "user-1", "Title")
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
		repo := threadpkg.NewThreadRepository(store)
		mgr := New(log, repo)

		threadID := "test-thread-fail"
		_, _, err := mgr.CreateThread(ctx, threadID, "user-1", "Title")
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

		if !errors.Is(err, threadpkg.ErrConcurrencyConflict) {
			t.Errorf("Expected concurrency conflict error, got: %v", err)
		}
	})
}

func TestManager_AutoTitling(t *testing.T) {
	ctx := context.Background()
	log := logger.New(io.Discard, logger.LevelInfo, "test", nil)
	store := NewConcurrentMockEventStore(0)
	repo := threadpkg.NewThreadRepository(store)
	mgr := New(log, repo)

	t.Run("Auto-titles on first user message", func(t *testing.T) {
		threadID := "auto-title-thread"

		// 1. Create thread with default title
		_, _, err := mgr.CreateThread(ctx, threadID, "user-1", "New Chat")
		if err != nil {
			t.Fatalf("Failed to create thread: %v", err)
		}

		// 2. Add first user message
		messageContent := "This is a long message that should be truncated for the title"
		_, _, _, err = mgr.AddUserMessage(ctx, threadID, "msg-1", messageContent, "user-1")
		if err != nil {
			t.Fatalf("Failed to add user message: %v", err)
		}

		// 3. Verify title was updated
		thread, _ := repo.Load(ctx, threadID)
		expectedTitle := messageContent[:50] + "..."
		if thread.Title != expectedTitle {
			t.Errorf("Expected title %q, got %q", expectedTitle, thread.Title)
		}

		// 4. Verify events
		streamName := threadpkg.StreamName("Thread", threadID)
		events, _ := store.ReadStream(ctx, streamName, 0)
		hasTitleUpdated := false
		for _, e := range events {
			if _, ok := e.(*threadpkg.ThreadTitleUpdated); ok {
				hasTitleUpdated = true
				break
			}
		}
		if !hasTitleUpdated {
			t.Error("Expected ThreadTitleUpdated event to be emitted")
		}
	})

	t.Run("Does not overwrite non-default title", func(t *testing.T) {
		threadID := "no-overwrite-thread"

		// 1. Create thread with custom title
		mgr.CreateThread(ctx, threadID, "user-1", "Custom Title")

		// 2. Add user message
		_, _, _, _ = mgr.AddUserMessage(ctx, threadID, "msg-1", "Some content", "user-1")

		// 3. Verify title remains the same
		thread, _ := repo.Load(ctx, threadID)
		if thread.Title != "Custom Title" {
			t.Errorf("Expected title 'Custom Title', got %q", thread.Title)
		}
	})
}
