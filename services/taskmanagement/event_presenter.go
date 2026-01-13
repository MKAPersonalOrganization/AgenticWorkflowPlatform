package taskmanagement

import (
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// UINotifier listens to domain events and notifies the UI
// This separates business logic (domain events) from presentation logic (UI events).
type UINotifier struct {
	emitter EventEmitter
}

// NewUINotifier creates a new UI notifier.
func NewUINotifier(emitter EventEmitter) *UINotifier {
	return &UINotifier{
		emitter: emitter,
	}
}

// HandleDomainEvent processes a domain event and emits corresponding UI events.
func (n *UINotifier) HandleDomainEvent(event core.Event) error {
	switch evt := event.(type) {
	case *domain.AssistantMessageStartedEvent:
		return n.emitTextMessageStart(evt)

	case *domain.AssistantContentStreamedEvent:
		return n.emitTextMessageContent(evt)

	case *domain.ToolCallStartedEvent:
		return n.emitToolCallStart(evt)

	case *domain.ToolCallArgumentsStreamedEvent:
		return n.emitToolCallArgs(evt)

	case *domain.ToolCallCompletedEvent:
		return n.emitToolCallEnd(evt)

	case *domain.AssistantMessageCompletedEvent:
		return n.emitTextMessageEnd(evt)

	case *domain.ToolCallExecutedEvent:
		return n.emitToolCallResult(evt)

	case *domain.ThreadTitleUpdatedEvent:
		return n.emitThreadTitleUpdated(evt)

	// Other domain events don't need UI events
	default:
		return nil
	}
}

func (n *UINotifier) emitThreadTitleUpdated(evt *domain.ThreadTitleUpdatedEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"THREAD_TITLE_UPDATED",
		aguievents.WithValue(map[string]any{
			"threadId": evt.ThreadID,
			"title":    evt.Title,
		}),
	)

	err := n.emitter.Emit(uiEvent)
	if err != nil {
		return fmt.Errorf("failed to emit thread title updated event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitTextMessageStart(evt *domain.AssistantMessageStartedEvent) error {
	uiEvent := aguievents.NewTextMessageStartEvent(
		evt.MessageID,
		aguievents.WithRole("assistant"),
	)

	err := n.emitter.Emit(uiEvent)
	if err != nil {
		return fmt.Errorf("failed to emit text message start event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitTextMessageContent(evt *domain.AssistantContentStreamedEvent) error {
	uiEvent := aguievents.NewTextMessageContentEvent(
		evt.MessageID,
		evt.Delta,
	)

	err := n.emitter.Emit(uiEvent)
	if err != nil {
		return fmt.Errorf("failed to emit text message content event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitToolCallStart(evt *domain.ToolCallStartedEvent) error {
	// First emit TOOL_CALL_AGENT to establish agent-tool call association
	agentEvent := aguievents.NewCustomEvent(
		"TOOL_CALL_AGENT",
		aguievents.WithValue(map[string]any{
			"toolCallId": evt.ToolCallID,
			"agentName":  evt.AgentName,
		}),
	)
	err := n.emitter.Emit(agentEvent)
	if err != nil {
		return fmt.Errorf("failed to emit tool call agent event: %w", err)
	}

	// Then emit standard TOOL_CALL_START event
	uiEvent := aguievents.NewToolCallStartEvent(
		evt.ToolCallID,
		evt.ToolCallName,
		aguievents.WithParentMessageID(evt.ParentMessageID),
	)

	if err := n.emitter.Emit(uiEvent); err != nil {
		return fmt.Errorf("failed to emit tool call start event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitToolCallArgs(evt *domain.ToolCallArgumentsStreamedEvent) error {
	uiEvent := aguievents.NewToolCallArgsEvent(
		evt.ToolCallID,
		evt.Delta,
	)

	err := n.emitter.Emit(uiEvent)
	if err != nil {
		return fmt.Errorf("failed to emit tool call args event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitToolCallEnd(evt *domain.ToolCallCompletedEvent) error {
	uiEvent := aguievents.NewToolCallEndEvent(
		evt.ToolCallID,
	)

	err := n.emitter.Emit(uiEvent)
	if err != nil {
		return fmt.Errorf("failed to emit tool call end event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitTextMessageEnd(evt *domain.AssistantMessageCompletedEvent) error {
	uiEvent := aguievents.NewTextMessageEndEvent(
		evt.MessageID,
	)

	err := n.emitter.Emit(uiEvent)
	if err != nil {
		return fmt.Errorf("failed to emit text message end event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitToolCallResult(evt *domain.ToolCallExecutedEvent) error {
	uiEvent := aguievents.NewToolCallResultEvent(
		evt.MessageID,
		evt.ToolCallID,
		evt.ToolResult,
	)

	err := n.emitter.Emit(uiEvent)
	if err != nil {
		return fmt.Errorf("failed to emit tool call result event: %w", err)
	}

	return nil
}
