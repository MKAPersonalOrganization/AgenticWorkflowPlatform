package eventstore

import (
	"context"
	"testing"
	"time"

	sdk "school-with-ai/sdk/agent-sdk/core"
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

	_, _ = store.Append(ctx, "thread-1", event1, event2, event3)

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
