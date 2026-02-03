package domain

import (
	"testing"
	"time"
)

// TestThreadDeciderCreateThread verifies the create thread command.
func TestThreadDeciderCreateThread(t *testing.T) {
	t.Parallel()

	// Using the new pattern - create thread directly
	thread, err := OpenThread("thread-1", "user-1", "My First Thread")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := thread.DequeueUncommittedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	created, ok := events[0].(*ThreadCreated)
	if !ok {
		t.Fatalf("expected ThreadCreated event, got %T", events[0])
	}

	if created.ThreadID != "thread-1" {
		t.Errorf("expected ThreadID 'thread-1', got '%s'", created.ThreadID)
	}

	if created.UserID != "user-1" {
		t.Errorf("expected UserID 'user-1', got '%s'", created.UserID)
	}

	if created.Title != "My First Thread" {
		t.Errorf("expected Title 'My First Thread', got '%s'", created.Title)
	}
}

// TestThreadDeciderCreateThreadValidation verifies validation rules.
func TestThreadDeciderCreateThreadValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		threadID string
		userID   string
		title    string
		wantErr  bool
	}{
		{
			name:     "missing thread ID",
			threadID: "",
			userID:   "user-1",
			wantErr:  true,
		},
		{
			name:     "missing user ID",
			threadID: "thread-1",
			userID:   "",
			wantErr:  true,
		},
		{
			name:     "valid command",
			threadID: "thread-1",
			userID:   "user-1",
			title:    "Test",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := OpenThread(tt.threadID, tt.userID, tt.title)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error=%v, got error=%v", tt.wantErr, err)
			}
		})
	}
}

// TestThreadDeciderAddMessage verifies adding messages.
func TestThreadDeciderAddMessage(t *testing.T) {
	t.Parallel()

	// Create thread first using the new pattern
	thread, _ := OpenThread("thread-1", "user-1", "Test")
	thread.DequeueUncommittedEvents() // Clear creation event

	// Now add a message
	err := thread.AddMessage("msg-1", "user", "Hello!", nil, "", "", "", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := thread.DequeueUncommittedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	added, ok := events[0].(*UserMessageAdded)
	if !ok {
		t.Fatalf("expected UserMessageAdded event, got %T", events[0])
	}

	if added.MessageID != "msg-1" {
		t.Errorf("expected MessageID 'msg-1', got '%s'", added.MessageID)
	}

	if added.Content != "Hello!" {
		t.Errorf("expected Content 'Hello!', got '%s'", added.Content)
	}
}

// TestThreadAggregateEvolve verifies state transitions.
func TestThreadAggregateEvolve(t *testing.T) {
	t.Parallel()
	thread := NewThreadAggregate("thread-1")

	// Apply ThreadCreated event
	created := &ThreadCreated{
		ThreadID:  "thread-1",
		UserID:    "user-1",
		Title:     "Test Thread",
		Timestamp: time.Now(),
	}
	thread.Apply(created)

	if thread.ID != "thread-1" {
		t.Errorf("expected ID 'thread-1', got '%s'", thread.ID)
	}

	if thread.UserID != "user-1" {
		t.Errorf("expected UserID 'user-1', got '%s'", thread.UserID)
	}

	if thread.Title != "Test Thread" {
		t.Errorf("expected Title 'Test Thread', got '%s'", thread.Title)
	}

	// Apply UserMessageAdded event
	msgAdded := &UserMessageAdded{
		ThreadID:  "thread-1",
		MessageID: "msg-1",
		Content:   "Hello!",
		Timestamp: time.Now(),
	}
	thread.Apply(msgAdded)

	if len(thread.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(thread.Messages))
	}

	if thread.Messages[0].ID != "msg-1" {
		t.Errorf("expected message ID 'msg-1', got '%s'", thread.Messages[0].ID)
	}

	if thread.Messages[0].Content != "Hello!" {
		t.Errorf("expected content 'Hello!', got '%s'", thread.Messages[0].Content)
	}
}

// TestThreadAggregateValidation verifies business rule validation.
func TestThreadAggregateValidation(t *testing.T) {
	t.Parallel()

	// Create and delete thread
	thread, _ := OpenThread("thread-1", "user-1", "Test")
	thread.DequeueUncommittedEvents()

	thread.Delete()
	thread.DequeueUncommittedEvents()

	// Try to add message to deleted thread - should fail
	err := thread.AddMessage("msg-1", "user", "Hello!", nil, "", "", "", nil)
	if err == nil {
		t.Error("expected error when adding message to deleted thread")
	}
}

// TestThreadTitleChange verifies title changes.
func TestThreadTitleChange(t *testing.T) {
	t.Parallel()

	// Create thread
	thread, _ := OpenThread("thread-1", "user-1", "Original Title")
	thread.DequeueUncommittedEvents()

	// Change title
	err := thread.ChangeTitle("New Title")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := thread.DequeueUncommittedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	changed, ok := events[0].(*ThreadTitleUpdated)
	if !ok {
		t.Fatalf("expected ThreadTitleUpdated event, got %T", events[0])
	}

	if changed.Title != "New Title" {
		t.Errorf("expected Title 'New Title', got '%s'", changed.Title)
	}

	// Verify state was updated
	if thread.Title != "New Title" {
		t.Errorf("expected title 'New Title', got '%s'", thread.Title)
	}
}

// TestThreadTitleChangeIdempotent verifies no event if title unchanged.
func TestThreadTitleChangeIdempotent(t *testing.T) {
	t.Parallel()

	// Create thread
	thread, _ := OpenThread("thread-1", "user-1", "Same Title")
	thread.DequeueUncommittedEvents()

	// Try to change to same title
	err := thread.ChangeTitle("Same Title")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return no events (idempotent)
	events := thread.DequeueUncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events for unchanged title, got %d", len(events))
	}
}

// TestAggregateVersionTracking verifies version tracking follows EventSourcing.NetCore pattern.
// Version should only track PERSISTED events, not uncommitted ones.
func TestAggregateVersionTracking(t *testing.T) {
	t.Parallel()

	thread, _ := OpenThread("thread-1", "user-1", "Test")

	// Version should be 0 - Enqueue doesn't increment version, only LoadFromHistory does
	if thread.Version != 0 {
		t.Errorf("expected version 0 after creation (uncommitted), got %d", thread.Version)
	}

	// Should have 1 uncommitted event
	uncommitted := thread.DequeueUncommittedEvents()
	if len(uncommitted) != 1 {
		t.Errorf("expected 1 uncommitted event, got %d", len(uncommitted))
	}

	// Simulate persisting and reloading (what repository does)
	thread.LoadFromHistory(uncommitted, thread.Apply)

	// NOW version should be 1 (1 persisted event)
	if thread.Version != 1 {
		t.Errorf("expected version 1 after loading history, got %d", thread.Version)
	}

	// Add another message
	thread.AddMessage("msg-1", "user", "Test", nil, "", "", "", nil)

	// Version still 1 (new event not persisted yet)
	if thread.Version != 1 {
		t.Errorf("expected version 1 after adding uncommitted message, got %d", thread.Version)
	}

	// Simulate persist and reload
	uncommitted2 := thread.DequeueUncommittedEvents()
	if len(uncommitted2) != 1 {
		t.Errorf("expected 1 uncommitted event, got %d", len(uncommitted2))
	}

	thread.LoadFromHistory(uncommitted2, thread.Apply)

	// NOW version should be 2
	if thread.Version != 2 {
		t.Errorf("expected version 2 after loading second event, got %d", thread.Version)
	}
}
