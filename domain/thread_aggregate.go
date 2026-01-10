package domain

import (
	"errors"
	"school-with-ai/sdk/agentsdk/core"
	"time"
)

// ThreadAggregate is an event-sourced aggregate representing a conversation thread.
// All state changes are represented as events in the event store.
type ThreadAggregate struct {
	AggregateState
	UserID    string
	Title     string
	Messages  []Message
	CreatedAt time.Time
	UpdatedAt time.Time
	IsDeleted bool
}

// NewThreadAggregate creates a new thread aggregate.
func NewThreadAggregate(id string) *ThreadAggregate {
	return &ThreadAggregate{
		AggregateState: NewAggregateState(id),
		Messages:       make([]Message, 0),
	}
}

// Evolve applies an event to update the thread's state.
// This is the state transition function - it's pure and has no side effects.
func (t *ThreadAggregate) Evolve(event core.Event) {
	switch e := event.(type) {
	case *ThreadCreated:
		t.ID = e.ThreadID
		t.UserID = e.UserID
		t.Title = e.Title
		t.CreatedAt = e.Timestamp
		t.UpdatedAt = e.Timestamp
		t.IsDeleted = false

	case ThreadCreated:
		t.ID = e.ThreadID
		t.UserID = e.UserID
		t.Title = e.Title
		t.CreatedAt = e.Timestamp
		t.UpdatedAt = e.Timestamp
		t.IsDeleted = false

	case *MessageAdded:
		msg := Message{
			ID:         e.MessageID,
			Role:       e.Role,
			Content:    e.Content,
			Timestamp:  e.Timestamp,
			ToolCalls:  e.ToolCalls,
			ToolCallID: e.ToolCallID,
			AgentName:  e.AgentName,
			Metadata:   e.Metadata,
		}
		t.Messages = append(t.Messages, msg)
		t.UpdatedAt = e.Timestamp

	case MessageAdded:
		msg := Message{
			ID:         e.MessageID,
			Role:       e.Role,
			Content:    e.Content,
			Timestamp:  e.Timestamp,
			ToolCalls:  e.ToolCalls,
			ToolCallID: e.ToolCallID,
			AgentName:  e.AgentName,
			Metadata:   e.Metadata,
		}
		t.Messages = append(t.Messages, msg)
		t.UpdatedAt = e.Timestamp

	case *ThreadTitleChanged:
		t.Title = e.NewTitle
		t.UpdatedAt = e.Timestamp

	case ThreadTitleChanged:
		t.Title = e.NewTitle
		t.UpdatedAt = e.Timestamp

	case *MessagesCleared:
		t.Messages = make([]Message, 0)
		t.UpdatedAt = e.Timestamp

	case MessagesCleared:
		t.Messages = make([]Message, 0)
		t.UpdatedAt = e.Timestamp

	case *ThreadDeleted:
		t.IsDeleted = true
		t.UpdatedAt = e.Timestamp

	case ThreadDeleted:
		t.IsDeleted = true
		t.UpdatedAt = e.Timestamp

	case *ToolCallExecuted:
		// Tool call execution doesn't modify aggregate state,
		// but we track it for projections (e.g., whiteboard)
		t.UpdatedAt = e.Timestamp

	case ToolCallExecuted:
		// Tool call execution doesn't modify aggregate state,
		// but we track it for projections (e.g., whiteboard)
		t.UpdatedAt = e.Timestamp
	}
}

// StreamName returns the event stream name for this aggregate.
func (t *ThreadAggregate) StreamName() string {
	return StreamName("Thread", t.ID)
}

// Validate checks if the thread is in a valid state.
func (t *ThreadAggregate) Validate() error {
	if t.IsDeleted {
		return errors.New("thread has been deleted")
	}
	return nil
}

// GetMessageCount returns the number of messages in the thread.
func (t *ThreadAggregate) GetMessageCount() int {
	return len(t.Messages)
}

// GetLastMessage returns the last message in the thread, or nil if empty.
func (t *ThreadAggregate) GetLastMessage() *Message {
	if len(t.Messages) == 0 {
		return nil
	}
	return &t.Messages[len(t.Messages)-1]
}
