package agents

import (
	"context"

	sdk "school-with-ai/sdk/agent-sdk/core"
)

// EventHandlerContext provides the context needed for event handlers to execute.
type EventHandlerContext struct {
	ThreadID string
	Event    *DomainEvent
	Agent    *sdk.AgentService
	State    sdk.ThreadStore
	Notifier sdk.EventNotifier
}

// EventHandler is a function that handles a specific event type for an agent.
// It owns the complete logic for reacting to the event, including:
// - Reading state and context
// - Executing agent logic (LLM calls, tool usage)
// - Updating state
// - Emitting resulting events
type EventHandler func(ctx context.Context, handlerCtx *EventHandlerContext) error

// EventHandlerRegistry maps event types to their handler functions.
type EventHandlerRegistry map[EventType]EventHandler

// Handle invokes the appropriate handler for an event type.
// Returns whether a handler was found and any error from execution.
func (r EventHandlerRegistry) Handle(ctx context.Context, eventType EventType, handlerCtx *EventHandlerContext) (bool, error) {
	handler, ok := r[eventType]
	if !ok {
		return false, nil
	}

	err := handler(ctx, handlerCtx)
	return true, err
}

// Register adds a handler for an event type.
func (r EventHandlerRegistry) Register(eventType EventType, handler EventHandler) {
	r[eventType] = handler
}
