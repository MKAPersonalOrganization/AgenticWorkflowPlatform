package domain

import (
	"errors"
	"fmt"
	"log/slog"
	"school-with-ai/sdk/agentsdk/core"
	"time"
)

// ThreadAggregate is an event-sourced aggregate representing a conversation thread.
// Following the pattern from EventSourcing.NetCore, business logic is implemented
// as methods on the aggregate that validate rules and enqueue events.
type ThreadAggregate struct {
	BaseAggregate
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
		BaseAggregate: BaseAggregate{
			ID:                id,
			Version:           0,
			uncommittedEvents: make([]core.Event, 0),
		},
		Messages: make([]Message, 0),
	}
}

// Open creates a new thread. This is a static factory method like ShoppingCart.Open.
func OpenThread(threadID, userID, title string) (*ThreadAggregate, error) {
	if threadID == "" {
		return nil, errors.New("thread ID is required")
	}
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	thread := NewThreadAggregate(threadID)
	event := &ThreadCreated{
		ThreadID:  threadID,
		UserID:    userID,
		Title:     title,
		Timestamp: time.Now(),
	}

	thread.Enqueue(event, thread.Apply)
	return thread, nil
}

// AddMessage adds a message to the thread.
// This implements business logic like ShoppingCart.AddProduct.
func (t *ThreadAggregate) AddMessage(messageID, role, content string, toolCalls []core.ToolCall, toolCallID, agentName string, metadata map[string]interface{}) error {
	if t.IsDeleted {
		return errors.New("cannot add message to deleted thread")
	}
	if messageID == "" {
		return errors.New("message ID is required")
	}
	if role == "" {
		return errors.New("message role is required")
	}

	event := &MessageAdded{
		ThreadID:   t.ID,
		MessageID:  messageID,
		Role:       role,
		Content:    content,
		ToolCalls:  toolCalls,
		ToolCallID: toolCallID,
		AgentName:  agentName,
		Metadata:   metadata,
		Timestamp:  time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// ChangeTitle changes the thread title.
func (t *ThreadAggregate) ChangeTitle(newTitle string) error {
	if t.IsDeleted {
		return errors.New("cannot change title of deleted thread")
	}

	// Idempotent - no event if title unchanged
	if t.Title == newTitle {
		return nil
	}

	event := &ThreadTitleChanged{
		ThreadID:  t.ID,
		NewTitle:  newTitle,
		Timestamp: time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// ClearMessages clears all messages from the thread.
func (t *ThreadAggregate) ClearMessages() error {
	if t.IsDeleted {
		return errors.New("cannot clear messages from deleted thread")
	}

	event := &MessagesCleared{
		ThreadID:  t.ID,
		Timestamp: time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// Delete marks the thread as deleted.
func (t *ThreadAggregate) Delete() error {
	if t.IsDeleted {
		return errors.New("thread already deleted")
	}

	event := &ThreadDeleted{
		ThreadID:  t.ID,
		Timestamp: time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// Apply applies an event to update the thread's state.
// This is the only place where state mutations happen.
// Handles both pointer and value types since events may be stored as values.
func (t *ThreadAggregate) Apply(event core.Event) {
	slog.Debug("Applying event to aggregate", "eventType", event.EventType(), "concreteType", fmt.Sprintf("%T", event))

	switch e := event.(type) {
	case *ThreadCreated:
		slog.Debug("Applying ThreadCreated", "pointer", true, "threadID", e.ThreadID, "title", e.Title)
		t.ID = e.ThreadID
		t.UserID = e.UserID
		t.Title = e.Title
		t.CreatedAt = e.Timestamp
		t.UpdatedAt = e.Timestamp
		t.IsDeleted = false

	case ThreadCreated: // Handle value type
		slog.Debug("Applying ThreadCreated", "pointer", false, "threadID", e.ThreadID, "title", e.Title)
		t.ID = e.ThreadID
		t.UserID = e.UserID
		t.Title = e.Title
		t.CreatedAt = e.Timestamp
		t.UpdatedAt = e.Timestamp
		t.IsDeleted = false

	case *MessageAdded:
		slog.Debug("Applying MessageAdded", "pointer", true, "messageID", e.MessageID, "role", e.Role)
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

	case MessageAdded: // Handle value type
		slog.Debug("Applying MessageAdded", "pointer", false, "messageID", e.MessageID, "role", e.Role)
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
		slog.Debug("Applying ThreadTitleChanged", "pointer", true, "newTitle", e.NewTitle)
		t.Title = e.NewTitle
		t.UpdatedAt = e.Timestamp

	case ThreadTitleChanged: // Handle value type
		slog.Debug("Applying ThreadTitleChanged", "pointer", false, "newTitle", e.NewTitle)
		t.Title = e.NewTitle
		t.UpdatedAt = e.Timestamp

	case *MessagesCleared:
		slog.Debug("Applying MessagesCleared", "pointer", true)
		t.Messages = make([]Message, 0)
		t.UpdatedAt = e.Timestamp

	case MessagesCleared: // Handle value type
		slog.Debug("Applying MessagesCleared", "pointer", false)
		t.Messages = make([]Message, 0)
		t.UpdatedAt = e.Timestamp

	case *ThreadDeleted:
		slog.Debug("Applying ThreadDeleted", "pointer", true)
		t.IsDeleted = true
		t.UpdatedAt = e.Timestamp

	case ThreadDeleted: // Handle value type
		slog.Debug("Applying ThreadDeleted", "pointer", false)
		t.IsDeleted = true
		t.UpdatedAt = e.Timestamp

	case *ToolCallExecuted:
		slog.Debug("Applying ToolCallExecuted", "pointer", true)
		// Tool call execution doesn't modify aggregate state,
		// but we track it for projections (e.g., whiteboard)
		t.UpdatedAt = e.Timestamp

	case ToolCallExecuted: // Handle value type
		slog.Debug("Applying ToolCallExecuted", "pointer", false)
		// Tool call execution doesn't modify aggregate state,
		// but we track it for projections (e.g., whiteboard)
		t.UpdatedAt = e.Timestamp

	default:
		slog.Warn("Unknown event type in Apply", "concreteType", fmt.Sprintf("%T", event))
	}
}

// StreamName returns the event stream name for this aggregate.
func (t *ThreadAggregate) StreamName() string {
	return StreamName("Thread", t.ID)
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
