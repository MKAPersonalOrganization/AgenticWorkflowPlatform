package orchestration

import (
	"fmt"
	"school-with-ai/sdk/agentsdk/core"

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
	case *core.AssistantMessageStartedEvent:
		return n.emitTextMessageStart(evt)

	case *core.AssistantContentStreamedEvent:
		return n.emitTextMessageContent(evt)

	case *core.ToolCallStartedEvent:
		return n.emitToolCallStart(evt)

	case *core.ToolCallArgumentsStreamedEvent:
		return n.emitToolCallArgs(evt)

	case *core.ToolCallCompletedEvent:
		return n.emitToolCallEnd(evt)

	case *core.AssistantMessageCompletedEvent:
		return n.emitTextMessageEnd(evt)

	case *core.ToolCallExecutedEvent:
		return n.emitToolCallResult(evt)

	// Other domain events don't need UI events
	default:
		return nil
	}
}

func (n *UINotifier) emitTextMessageStart(evt *core.AssistantMessageStartedEvent) error {
	uiEvent := aguievents.NewTextMessageStartEvent(
		evt.MessageID,
		aguievents.WithRole("assistant"),
	)

	if err := n.emitter.Emit(uiEvent); err != nil {
		return fmt.Errorf("failed to emit text message start event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitTextMessageContent(evt *core.AssistantContentStreamedEvent) error {
	uiEvent := aguievents.NewTextMessageContentEvent(
		evt.MessageID,
		evt.Delta,
	)

	if err := n.emitter.Emit(uiEvent); err != nil {
		return fmt.Errorf("failed to emit text message content event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitToolCallStart(evt *core.ToolCallStartedEvent) error {
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

func (n *UINotifier) emitToolCallArgs(evt *core.ToolCallArgumentsStreamedEvent) error {
	uiEvent := aguievents.NewToolCallArgsEvent(
		evt.ToolCallID,
		evt.Delta,
	)

	if err := n.emitter.Emit(uiEvent); err != nil {
		return fmt.Errorf("failed to emit tool call args event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitToolCallEnd(evt *core.ToolCallCompletedEvent) error {
	uiEvent := aguievents.NewToolCallEndEvent(
		evt.ToolCallID,
	)

	if err := n.emitter.Emit(uiEvent); err != nil {
		return fmt.Errorf("failed to emit tool call end event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitTextMessageEnd(evt *core.AssistantMessageCompletedEvent) error {
	uiEvent := aguievents.NewTextMessageEndEvent(
		evt.MessageID,
	)

	if err := n.emitter.Emit(uiEvent); err != nil {
		return fmt.Errorf("failed to emit text message end event: %w", err)
	}

	return nil
}

func (n *UINotifier) emitToolCallResult(evt *core.ToolCallExecutedEvent) error {
	uiEvent := aguievents.NewToolCallResultEvent(
		evt.MessageID,
		evt.ToolCallID,
		evt.ToolResult,
	)

	if err := n.emitter.Emit(uiEvent); err != nil {
		return fmt.Errorf("failed to emit tool call result event: %w", err)
	}

	return nil
}
