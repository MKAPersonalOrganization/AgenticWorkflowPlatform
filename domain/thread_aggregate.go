package domain

import (
	"errors"
	"time"
)

// ThreadAggregate is an event-sourced aggregate representing a conversation thread.
// Following the pattern from EventSourcing.NetCore, business logic is implemented
// as methods on the aggregate that validate rules and enqueue events.
// ThreadAggregate is an event-sourced aggregate representing a conversation thread.
// Following the pattern from EventSourcing.NetCore, business logic is implemented
// as methods on the aggregate that validate rules and enqueue events.
type ThreadAggregate struct {
	BaseAggregate
	UserID           string
	Title            string
	Messages         []Message
	ActiveToolStates map[string]ToolCallState
	CreatedAt        time.Time
	UpdatedAt        time.Time
	IsDeleted        bool
	EngineState      string
}

// NewThreadAggregate creates a new thread aggregate.
func NewThreadAggregate(id string) *ThreadAggregate {
	return &ThreadAggregate{
		BaseAggregate: BaseAggregate{
			ID:                id,
			Version:           0,
			uncommittedEvents: make([]Event, 0),
		},
		Messages:         make([]Message, 0),
		ActiveToolStates: make(map[string]ToolCallState),
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
	event := ThreadCreated{
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
func (t *ThreadAggregate) AddMessage(messageID, role, content string, toolCalls []ToolCall, toolCallID, toolName, agentName string, metadata map[string]interface{}) error {
	if t.IsDeleted {
		return errors.New("cannot add message to deleted thread")
	}
	if messageID == "" {
		return errors.New("message ID is required")
	}
	if role == "" {
		return errors.New("message role is required")
	}

	// Emit specific event based on role
	var event Event
	switch role {
	case "user":
		event = UserMessageAdded{
			ThreadID:  t.ID,
			MessageID: messageID,
			Content:   content,
			Timestamp: time.Now(),
		}
	case "assistant":
		event = AssistantMessageAdded{
			ThreadID:  t.ID,
			MessageID: messageID,
			Content:   content,
			ToolCalls: toolCalls,
			AgentName: agentName,
			Timestamp: time.Now(),
		}
	case "tool":
		event = ToolResultAdded{
			ThreadID:   t.ID,
			MessageID:  messageID,
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Result:     content,
			AgentName:  agentName,
			Timestamp:  time.Now(),
		}
	default:
		return errors.New("unsupported message role: " + role)
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

	event := ThreadTitleUpdated{
		ThreadID:  t.ID,
		Title:     newTitle,
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

	event := ThreadDeleted{
		ThreadID:  t.ID,
		Timestamp: time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// RecordUsage records token usage for a message in the thread.
func (t *ThreadAggregate) RecordUsage(event TokenUsageReported) error {
	if t.IsDeleted {
		return errors.New("cannot record usage for deleted thread")
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// RecordToolEvent records a tool lifecycle event in the thread.
func (t *ThreadAggregate) RecordToolEvent(event Event) error {
	if t.IsDeleted {
		return errors.New("cannot record tool event for deleted thread")
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// RecordEngineStateChange records a change in the engine's high-level state.
func (t *ThreadAggregate) RecordEngineStateChange(event EngineStateChanged) error {
	if t.IsDeleted {
		return errors.New("cannot record state change for deleted thread")
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// Apply applies an event to update the thread's state.
// This is the only place where state mutations happen.
func (t *ThreadAggregate) Apply(event Event) {
	switch e := event.(type) {
	case ThreadCreated:
		t.ID = e.ThreadID
		t.UserID = e.UserID
		t.Title = e.Title
		t.CreatedAt = e.Timestamp
		t.UpdatedAt = e.Timestamp
		t.IsDeleted = false

	case UserMessageAdded:
		msg := Message{
			ID:        e.MessageID,
			Role:      "user",
			Content:   e.Content,
			Timestamp: e.Timestamp,
		}
		t.Messages = append(t.Messages, msg)
		t.UpdatedAt = e.Timestamp

	case AssistantMessageAdded:
		msg := Message{
			ID:        e.MessageID,
			Role:      "assistant",
			Content:   e.Content,
			ToolCalls: e.ToolCalls,
			AgentName: e.AgentName,
			Timestamp: e.Timestamp,
		}
		t.Messages = append(t.Messages, msg)
		t.UpdatedAt = e.Timestamp

	case ToolResultAdded:
		msg := Message{
			ID:         e.MessageID,
			Role:       "tool",
			Content:    e.Result,
			ToolCallID: e.ToolCallID,
			ToolName:   e.ToolName,
			AgentName:  e.AgentName,
			Timestamp:  e.Timestamp,
		}
		t.Messages = append(t.Messages, msg)
		t.UpdatedAt = e.Timestamp

		// Remove the tool from active states when result is added
		delete(t.ActiveToolStates, e.ToolCallID)

	case ThreadTitleUpdated:
		t.Title = e.Title
		t.UpdatedAt = e.Timestamp

	case ThreadDeleted:
		t.IsDeleted = true
		t.UpdatedAt = e.Timestamp

	case ToolNeedsInput:
		state := ToolCallState{
			ToolCallID: e.ToolCallID,
			Status:     "AwaitingInput",
			Elicitation: map[string]any{
				"question": e.Elicitation.Question,
				"schema":   e.Elicitation.Schema,
			},
		}
		t.ActiveToolStates[e.ToolCallID] = state

	case ToolNeedsAuth:
		state := ToolCallState{
			ToolCallID: e.ToolCallID,
			Status:     "AwaitingAuth",
			AuthSpec: map[string]any{
				"resource": e.AuthSpec.Resource,
				"action":   e.AuthSpec.Action,
				"reason":   e.AuthSpec.Reason,
				"scopes":   e.AuthSpec.Scopes,
			},
		}
		t.ActiveToolStates[e.ToolCallID] = state

	case InputProvided:
		if state, ok := t.ActiveToolStates[e.ToolCallID]; ok {
			state.Status = "InputProvided"
			t.ActiveToolStates[e.ToolCallID] = state
		}

	case AuthGranted:
		if state, ok := t.ActiveToolStates[e.ToolCallID]; ok {
			state.Status = "AuthGranted"
			t.ActiveToolStates[e.ToolCallID] = state
		}

	case EngineStateChanged:
		t.EngineState = e.NewState
		t.UpdatedAt = e.Timestamp
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
