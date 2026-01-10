package domain

import (
	"testing"
	"time"

	"school-with-ai/sdk/agentsdk/core"
)

// TestThreadDeciderCreateThread verifies the create thread command.
func TestThreadDeciderCreateThread(t *testing.T) {
	t.Parallel()
	decider := NewThreadDecider()
	thread := NewThreadAggregate("thread-1")

	cmd := CreateThread{
		ThreadID: "thread-1",
		UserID:   "user-1",
		Title:    "My First Thread",
	}

	events, err := decider.Decide(cmd, thread)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

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
	decider := NewThreadDecider()
	thread := NewThreadAggregate("")

	tests := []struct {
		name    string
		cmd     CreateThread
		wantErr bool
	}{
		{
			name: "missing thread ID",
			cmd: CreateThread{
				ThreadID: "",
				UserID:   "user-1",
			},
			wantErr: true,
		},
		{
			name: "missing user ID",
			cmd: CreateThread{
				ThreadID: "thread-1",
				UserID:   "",
			},
			wantErr: true,
		},
		{
			name: "valid command",
			cmd: CreateThread{
				ThreadID: "thread-1",
				UserID:   "user-1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := decider.Decide(tt.cmd, thread)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error=%v, got error=%v", tt.wantErr, err)
			}
		})
	}
}

// TestThreadDeciderAddMessage verifies adding messages.
func TestThreadDeciderAddMessage(t *testing.T) {
	t.Parallel()
	decider := NewThreadDecider()

	// Create thread first
	thread := NewThreadAggregate("thread-1")
	createCmd := CreateThread{
		ThreadID: "thread-1",
		UserID:   "user-1",
		Title:    "Test",
	}

	events, _ := decider.Decide(createCmd, thread)
	for _, e := range events {
		thread.Evolve(e)
		thread.IncrementVersion()
	}

	// Now add a message
	addCmd := AddMessage{
		ThreadID:  "thread-1",
		MessageID: "msg-1",
		Role:      "user",
		Content:   "Hello!",
	}

	events, err := decider.Decide(addCmd, thread)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	added, ok := events[0].(*MessageAdded)
	if !ok {
		t.Fatalf("expected MessageAdded event, got %T", events[0])
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
	thread.Evolve(created)

	if thread.ID != "thread-1" {
		t.Errorf("expected ID 'thread-1', got '%s'", thread.ID)
	}

	if thread.UserID != "user-1" {
		t.Errorf("expected UserID 'user-1', got '%s'", thread.UserID)
	}

	if thread.Title != "Test Thread" {
		t.Errorf("expected Title 'Test Thread', got '%s'", thread.Title)
	}

	// Apply MessageAdded event
	msgAdded := &MessageAdded{
		ThreadID:  "thread-1",
		MessageID: "msg-1",
		Role:      "user",
		Content:   "Hello!",
		Timestamp: time.Now(),
	}
	thread.Evolve(msgAdded)

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
	thread := NewThreadAggregate("thread-1")

	// Create and delete thread
	thread.Evolve(&ThreadCreated{
		ThreadID:  "thread-1",
		UserID:    "user-1",
		Timestamp: time.Now(),
	})

	thread.Evolve(&ThreadDeleted{
		ThreadID:  "thread-1",
		Timestamp: time.Now(),
	})

	// Validation should fail for deleted thread
	err := thread.Validate()
	if err == nil {
		t.Error("expected validation error for deleted thread")
	}
}

// TestThreadTitleChange verifies title changes.
func TestThreadTitleChange(t *testing.T) {
	t.Parallel()
	decider := NewThreadDecider()
	thread := NewThreadAggregate("thread-1")

	// Create thread
	thread.Evolve(&ThreadCreated{
		ThreadID:  "thread-1",
		UserID:    "user-1",
		Title:     "Original Title",
		Timestamp: time.Now(),
	})

	// Change title
	cmd := ChangeThreadTitle{
		ThreadID: "thread-1",
		NewTitle: "New Title",
	}

	events, err := decider.Decide(cmd, thread)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	changed, ok := events[0].(*ThreadTitleChanged)
	if !ok {
		t.Fatalf("expected ThreadTitleChanged event, got %T", events[0])
	}

	if changed.NewTitle != "New Title" {
		t.Errorf("expected NewTitle 'New Title', got '%s'", changed.NewTitle)
	}

	// Apply event and verify state
	thread.Evolve(changed)
	if thread.Title != "New Title" {
		t.Errorf("expected title 'New Title', got '%s'", thread.Title)
	}
}

// TestThreadTitleChangeIdempotent verifies no event if title unchanged.
func TestThreadTitleChangeIdempotent(t *testing.T) {
	t.Parallel()
	decider := NewThreadDecider()
	thread := NewThreadAggregate("thread-1")

	// Create thread
	thread.Evolve(&ThreadCreated{
		ThreadID:  "thread-1",
		UserID:    "user-1",
		Title:     "Same Title",
		Timestamp: time.Now(),
	})

	// Try to change to same title
	cmd := ChangeThreadTitle{
		ThreadID: "thread-1",
		NewTitle: "Same Title",
	}

	events, err := decider.Decide(cmd, thread)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should return no events (idempotent)
	if len(events) != 0 {
		t.Errorf("expected 0 events for unchanged title, got %d", len(events))
	}
}

// TestAggregateVersionTracking verifies version increments.
func TestAggregateVersionTracking(t *testing.T) {
	t.Parallel()
	thread := NewThreadAggregate("thread-1")

	if thread.Version != 0 {
		t.Errorf("expected initial version 0, got %d", thread.Version)
	}

	events := []core.Event{
		&ThreadCreated{
			ThreadID:  "thread-1",
			UserID:    "user-1",
			Timestamp: time.Now(),
		},
		&MessageAdded{
			ThreadID:  "thread-1",
			MessageID: "msg-1",
			Role:      "user",
			Content:   "Test",
			Timestamp: time.Now(),
		},
	}

	for _, e := range events {
		thread.Evolve(e)
		thread.IncrementVersion()
	}

	if thread.Version != 2 {
		t.Errorf("expected version 2 after 2 events, got %d", thread.Version)
	}
}
