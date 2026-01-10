package domain

import (
	"errors"
	"time"

	"school-with-ai/sdk/agentsdk/core"
)

// ThreadDecider implements the business logic for Thread aggregate.
// It decides which events to emit based on commands and current state.

// ThreadCommand is a union type for all thread commands.
type ThreadCommand interface {
	isThreadCommand()
}

func (CreateThread) isThreadCommand()      {}
func (AddMessage) isThreadCommand()        {}
func (ChangeThreadTitle) isThreadCommand() {}
func (ClearMessages) isThreadCommand()     {}
func (DeleteThread) isThreadCommand()      {}

// NewThreadDecider creates a decider for Thread aggregate.
func NewThreadDecider() *Decider[*ThreadAggregate, ThreadCommand, core.Event] {
	return NewDecider(
		decideThreadCommand,
		evolveThreadEvent,
		func() *ThreadAggregate { return NewThreadAggregate("") },
	)
}

// decideThreadCommand is the decision function for thread commands.
// It validates business rules and returns events to emit.
func decideThreadCommand(cmd ThreadCommand, state *ThreadAggregate) ([]core.Event, error) {
	switch c := cmd.(type) {
	case CreateThread:
		return handleCreateThread(c, state)
	case AddMessage:
		return handleAddMessage(c, state)
	case ChangeThreadTitle:
		return handleChangeThreadTitle(c, state)
	case ClearMessages:
		return handleClearMessages(c, state)
	case DeleteThread:
		return handleDeleteThread(c, state)
	default:
		return nil, errors.New("unknown command type")
	}
}

// evolveThreadEvent applies an event to the thread state.
func evolveThreadEvent(state *ThreadAggregate, event core.Event) *ThreadAggregate {
	state.Evolve(event)
	return state
}

// Command handlers - these validate business rules and emit events

func handleCreateThread(cmd CreateThread, state *ThreadAggregate) ([]core.Event, error) {
	// Business rule: Thread cannot be created if it already exists
	// We check the version - if it's > 0, events have been applied, so thread exists
	if state.Version > 0 {
		return nil, errors.New("thread already exists")
	}

	if cmd.ThreadID == "" {
		return nil, errors.New("thread ID is required")
	}

	if cmd.UserID == "" {
		return nil, errors.New("user ID is required")
	}

	event := &ThreadCreated{
		ThreadID:  cmd.ThreadID,
		UserID:    cmd.UserID,
		Title:     cmd.Title,
		Timestamp: time.Now(),
	}

	return []Event{event}, nil
}

func handleAddMessage(cmd AddMessage, state *ThreadAggregate) ([]core.Event, error) {
	// Business rule: Cannot add messages to deleted thread
	if err := state.Validate(); err != nil {
		return nil, err
	}

	if cmd.MessageID == "" {
		return nil, errors.New("message ID is required")
	}

	if cmd.Role == "" {
		return nil, errors.New("message role is required")
	}

	event := &MessageAdded{
		ThreadID:   cmd.ThreadID,
		MessageID:  cmd.MessageID,
		Role:       cmd.Role,
		Content:    cmd.Content,
		ToolCalls:  cmd.ToolCalls,
		ToolCallID: cmd.ToolCallID,
		AgentName:  cmd.AgentName,
		Metadata:   cmd.Metadata,
		Timestamp:  time.Now(),
	}

	return []core.Event{event}, nil
}

func handleChangeThreadTitle(cmd ChangeThreadTitle, state *ThreadAggregate) ([]core.Event, error) {
	// Business rule: Cannot change title of deleted thread
	if err := state.Validate(); err != nil {
		return nil, err
	}

	// No-op if title hasn't changed
	if state.Title == cmd.NewTitle {
		return []core.Event{}, nil
	}

	event := &ThreadTitleChanged{
		ThreadID:  cmd.ThreadID,
		NewTitle:  cmd.NewTitle,
		Timestamp: time.Now(),
	}

	return []core.Event{event}, nil
}

func handleClearMessages(cmd ClearMessages, state *ThreadAggregate) ([]core.Event, error) {
	// Business rule: Cannot clear messages from deleted thread
	if err := state.Validate(); err != nil {
		return nil, err
	}

	// No-op if already empty
	if len(state.Messages) == 0 {
		return []core.Event{}, nil
	}

	event := &MessagesCleared{
		ThreadID:  cmd.ThreadID,
		Timestamp: time.Now(),
	}

	return []core.Event{event}, nil
}

func handleDeleteThread(cmd DeleteThread, state *ThreadAggregate) ([]core.Event, error) {
	// Business rule: Cannot delete already deleted thread
	if state.IsDeleted {
		return nil, errors.New("thread already deleted")
	}

	event := &ThreadDeleted{
		ThreadID:  cmd.ThreadID,
		Timestamp: time.Now(),
	}

	return []core.Event{event}, nil
}
