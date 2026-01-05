package eventstore

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"school-with-ai/sdk/agent-sdk/core"
)

func TestInMemoryEventStore_Append(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	t.Run("appends events to stream", func(t *testing.T) {
		event := &sdk.ThreadCreatedEvent{
			BaseEvent: sdk.BaseEvent{
				Type:      "ThreadCreated",
				Timestamp: time.Now(),
				ThreadID:  "thread-1",
			},
			Title: "Test Thread",
		}

		version, err := store.Append(ctx, "thread-1", event)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}

		if version != 1 {
			t.Errorf("expected version 1, got %d", version)
		}
	})

	t.Run("increments version on each append", func(t *testing.T) {
		event1 := &sdk.UserMessageAddedEvent{
			BaseEvent: sdk.BaseEvent{ThreadID: "thread-2"},
			Content:   "Hello",
		}
		event2 := &sdk.UserMessageAddedEvent{
			BaseEvent: sdk.BaseEvent{ThreadID: "thread-2"},
			Content:   "World",
		}

		v1, _ := store.Append(ctx, "thread-2", event1)
		v2, _ := store.Append(ctx, "thread-2", event2)

		if v1 != 1 {
			t.Errorf("expected first version 1, got %d", v1)
		}
		if v2 != 2 {
			t.Errorf("expected second version 2, got %d", v2)
		}
	})
}

func TestInMemoryEventStore_ReadStream(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	// Add some events
	event1 := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}, Content: "1"}
	event2 := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}, Content: "2"}
	event3 := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}, Content: "3"}

	store.Append(ctx, "thread-1", event1, event2, event3)

	t.Run("reads all events from beginning", func(t *testing.T) {
		events, err := store.ReadStream(ctx, "thread-1", 0)
		if err != nil {
			t.Fatalf("ReadStream failed: %v", err)
		}

		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
	})

	t.Run("reads events from specific version", func(t *testing.T) {
		events, err := store.ReadStream(ctx, "thread-1", 1)
		if err != nil {
			t.Fatalf("ReadStream failed: %v", err)
		}

		if len(events) != 2 {
			t.Errorf("expected 2 events from version 1, got %d", len(events))
		}
	})

	t.Run("returns empty for nonexistent stream", func(t *testing.T) {
		events, err := store.ReadStream(ctx, "nonexistent", 0)
		if err != nil {
			t.Fatalf("ReadStream failed: %v", err)
		}

		if len(events) != 0 {
			t.Errorf("expected 0 events for nonexistent stream, got %d", len(events))
		}
	})
}

func TestInMemoryEventStore_StreamVersion(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	t.Run("returns 0 for nonexistent stream", func(t *testing.T) {
		version, err := store.StreamVersion(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("StreamVersion failed: %v", err)
		}

		if version != 0 {
			t.Errorf("expected version 0, got %d", version)
		}
	})

	t.Run("returns correct version after appends", func(t *testing.T) {
		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}}
		store.Append(ctx, "thread-1", event, event, event)

		version, _ := store.StreamVersion(ctx, "thread-1")
		if version != 3 {
			t.Errorf("expected version 3, got %d", version)
		}
	})
}

func TestInMemoryEventStore_StreamExists(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	t.Run("returns false for nonexistent stream", func(t *testing.T) {
		exists, _ := store.StreamExists(ctx, "nonexistent")
		if exists {
			t.Error("expected false for nonexistent stream")
		}
	})

	t.Run("returns true after append", func(t *testing.T) {
		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}}
		store.Append(ctx, "thread-1", event)

		exists, _ := store.StreamExists(ctx, "thread-1")
		if !exists {
			t.Error("expected true after append")
		}
	})
}

func TestInMemoryEventStore_DeleteStream(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}}
	store.Append(ctx, "thread-1", event)

	err := store.DeleteStream(ctx, "thread-1")
	if err != nil {
		t.Fatalf("DeleteStream failed: %v", err)
	}

	exists, _ := store.StreamExists(ctx, "thread-1")
	if exists {
		t.Error("expected stream to be deleted")
	}
}

func TestInMemoryEventStore_ListStreams(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	t.Run("returns empty list initially", func(t *testing.T) {
		streams, _ := store.ListStreams(ctx)
		if len(streams) != 0 {
			t.Errorf("expected 0 streams, got %d", len(streams))
		}
	})

	t.Run("returns all streams", func(t *testing.T) {
		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{}}
		store.Append(ctx, "stream-a", event)
		store.Append(ctx, "stream-b", event)
		store.Append(ctx, "stream-c", event)

		streams, _ := store.ListStreams(ctx)
		if len(streams) != 3 {
			t.Errorf("expected 3 streams, got %d", len(streams))
		}
	})
}

func TestInMemoryEventStore_OptimisticLocking(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	t.Run("succeeds with correct expected version", func(t *testing.T) {
		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}}

		// First append with expected version -1 (create new)
		v1, err := store.AppendWithExpectedVersion(ctx, "thread-1", -1, event)
		if err != nil {
			t.Fatalf("First append failed: %v", err)
		}
		if v1 != 1 {
			t.Errorf("expected version 1, got %d", v1)
		}

		// Second append with expected version 1
		v2, err := store.AppendWithExpectedVersion(ctx, "thread-1", 1, event)
		if err != nil {
			t.Fatalf("Second append failed: %v", err)
		}
		if v2 != 2 {
			t.Errorf("expected version 2, got %d", v2)
		}
	})

	t.Run("fails with wrong expected version", func(t *testing.T) {
		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-2"}}
		store.Append(ctx, "thread-2", event)

		// Try to append with wrong expected version
		_, err := store.AppendWithExpectedVersion(ctx, "thread-2", 0, event)
		if err != sdk.ErrConcurrencyConflict {
			t.Errorf("expected sdk.ErrConcurrencyConflict, got %v", err)
		}
	})

	t.Run("fails creating existing stream with -1", func(t *testing.T) {
		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-3"}}
		store.Append(ctx, "thread-3", event)

		// Try to create with -1 when stream exists
		_, err := store.AppendWithExpectedVersion(ctx, "thread-3", -1, event)
		if err != sdk.ErrConcurrencyConflict {
			t.Errorf("expected sdk.ErrConcurrencyConflict, got %v", err)
		}
	})
}

func TestInMemoryEventStore_Subscribe(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	t.Run("notifies subscriber of new events", func(t *testing.T) {
		var receivedCount int32

		unsubscribe, err := store.Subscribe(ctx, "thread-1", func(ctx context.Context, streamID string, event sdk.Event) error {
			atomic.AddInt32(&receivedCount, 1)
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}
		defer unsubscribe()

		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-1"}}
		store.Append(ctx, "thread-1", event, event, event)

		// Give time for async handlers
		time.Sleep(10 * time.Millisecond)

		if atomic.LoadInt32(&receivedCount) != 3 {
			t.Errorf("expected 3 notifications, got %d", receivedCount)
		}
	})

	t.Run("unsubscribe stops notifications", func(t *testing.T) {
		var receivedCount int32

		unsubscribe, _ := store.Subscribe(ctx, "thread-2", func(ctx context.Context, streamID string, event sdk.Event) error {
			atomic.AddInt32(&receivedCount, 1)
			return nil
		})

		event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{ThreadID: "thread-2"}}
		store.Append(ctx, "thread-2", event)

		unsubscribe()

		store.Append(ctx, "thread-2", event)

		time.Sleep(10 * time.Millisecond)

		if atomic.LoadInt32(&receivedCount) != 1 {
			t.Errorf("expected 1 notification after unsubscribe, got %d", receivedCount)
		}
	})
}

func TestInMemoryEventStore_SubscribeAll(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	var receivedStreams []string
	var mu sync.Mutex

	unsubscribe, _ := store.SubscribeAll(ctx, func(ctx context.Context, streamID string, event sdk.Event) error {
		mu.Lock()
		receivedStreams = append(receivedStreams, streamID)
		mu.Unlock()
		return nil
	})
	defer unsubscribe()

	event := &sdk.UserMessageAddedEvent{BaseEvent: sdk.BaseEvent{}}
	store.Append(ctx, "stream-a", event)
	store.Append(ctx, "stream-b", event)
	store.Append(ctx, "stream-c", event)

	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(receivedStreams) != 3 {
		t.Errorf("expected 3 notifications from different streams, got %d", len(receivedStreams))
	}
	mu.Unlock()
}

func TestInMemoryEventStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			event := &sdk.UserMessageAddedEvent{
				BaseEvent: sdk.BaseEvent{ThreadID: "shared"},
				Content:   "message",
			}
			store.Append(ctx, "shared", event)
		}(i)
	}

	wg.Wait()

	version, _ := store.StreamVersion(ctx, "shared")
	if version != int64(numGoroutines) {
		t.Errorf("expected version %d, got %d", numGoroutines, version)
	}
}
