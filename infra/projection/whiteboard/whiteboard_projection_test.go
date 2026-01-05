package whiteboard

import (
	"testing"
	"time"

	"school-with-ai/services/agents"
)

func TestWhiteboardProjection_Write(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	event := agents.NewWhiteboardUpdatedEvent(threadID, "test-agent", "Hello, World!", "write")
	err := store.Update(event)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	content := store.Get(threadID)
	if content != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got '%s'", content)
	}
}

func TestWhiteboardProjection_Append(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	event1 := agents.NewWhiteboardUpdatedEvent(threadID, "test-agent", "First line", "write")
	store.Update(event1)

	event2 := agents.NewWhiteboardUpdatedEvent(threadID, "test-agent", "Second line", "append")
	store.Update(event2)

	content := store.Get(threadID)
	expected := "First line\nSecond line"
	if content != expected {
		t.Errorf("Expected '%s', got '%s'", expected, content)
	}
}

func TestWhiteboardProjection_AppendToEmpty(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	event := agents.NewWhiteboardUpdatedEvent(threadID, "test-agent", "First line", "append")
	store.Update(event)

	content := store.Get(threadID)
	if content != "First line" {
		t.Errorf("Expected 'First line', got '%s'", content)
	}
}

func TestWhiteboardProjection_Clear(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	event1 := agents.NewWhiteboardUpdatedEvent(threadID, "test-agent", "Some content", "write")
	store.Update(event1)

	event2 := agents.NewWhiteboardUpdatedEvent(threadID, "test-agent", "", "clear")
	store.Update(event2)

	content := store.Get(threadID)
	if content != "" {
		t.Errorf("Expected empty string, got '%s'", content)
	}
}

func TestWhiteboardProjection_MultipleThreads(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()

	event1 := agents.NewWhiteboardUpdatedEvent("thread-1", "agent-1", "Thread 1 content", "write")
	store.Update(event1)

	event2 := agents.NewWhiteboardUpdatedEvent("thread-2", "agent-2", "Thread 2 content", "write")
	store.Update(event2)

	content1 := store.Get("thread-1")
	if content1 != "Thread 1 content" {
		t.Errorf("Thread 1: Expected 'Thread 1 content', got '%s'", content1)
	}

	content2 := store.Get("thread-2")
	if content2 != "Thread 2 content" {
		t.Errorf("Thread 2: Expected 'Thread 2 content', got '%s'", content2)
	}

	all := store.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 whiteboards, got %d", len(all))
	}
}

func TestWhiteboardProjection_NonExistentThread(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()

	content := store.Get("non-existent")
	if content != "" {
		t.Errorf("Expected empty string for non-existent thread, got '%s'", content)
	}
}

func TestWhiteboardProjection_IgnoreNonWhiteboardEvents(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	nonWhiteboardEvent := &agents.DomainEvent{
		ID:        "event-1",
		Type:      agents.EventTypeUserMessage,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Data: map[string]interface{}{
			"content": "This should be ignored",
		},
	}

	err := store.Update(nonWhiteboardEvent)
	if err != nil {
		t.Fatalf("Update should not fail for non-whiteboard events: %v", err)
	}

	content := store.Get(threadID)
	if content != "" {
		t.Errorf("Expected empty string, got '%s'", content)
	}
}

func TestWhiteboardProjection_UnknownAction(t *testing.T) {
	store := NewInMemoryWhiteboardProjection()
	threadID := "thread-123"

	event1 := agents.NewWhiteboardUpdatedEvent(threadID, "test-agent", "Initial content", "write")
	store.Update(event1)

	event2 := &agents.DomainEvent{
		ID:        "event-2",
		Type:      agents.EventTypeWhiteboardUpdated,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Data: map[string]interface{}{
			"action":  "unknown_action",
			"content": "This should be ignored",
		},
	}
	store.Update(event2)

	content := store.Get(threadID)
	if content != "Initial content" {
		t.Errorf("Expected 'Initial content', got '%s'", content)
	}
}
