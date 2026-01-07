package orchestration

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// UINotifier listens to domain events and notifies the UI
// This separates business logic (domain events) from presentation logic (UI events)
type UINotifier struct {
	emitter EventEmitter
}

// NewUINotifier creates a new UI notifier
func NewUINotifier(emitter EventEmitter) *UINotifier {
	return &UINotifier{
		emitter: emitter,
	}
}

// HandleDomainEvent processes a domain event and emits corresponding UI events
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

	// Agent collaboration events
	case *agents.DomainEvent:
		// Handle unified domain events
		switch evt.Type {
		case agents.EventTypeMathQuestion:
			return n.emitMathQuestion(evt)
		case agents.EventTypePhysicsQuestion:
			return n.emitPhysicsQuestion(evt)
		case agents.EventTypeChemistryQuestion:
			return n.emitChemistryQuestion(evt)
		case agents.EventTypeStatisticsQuestion:
			return n.emitStatisticsQuestion(evt)
		case agents.EventTypeLinearAlgebraQuestion:
			return n.emitLinearAlgebraQuestion(evt)
		case agents.EventTypeDiagrammingQuestion:
			return n.emitDiagrammingQuestion(evt)
		case agents.EventTypeQuestionAnswered:
			return n.emitQuestionAnswered(evt)
		case agents.EventTypeAgentHandoff:
			return n.emitAgentHandoff(evt)
		default:
			return nil
		}

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
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitTextMessageContent(evt *core.AssistantContentStreamedEvent) error {
	uiEvent := aguievents.NewTextMessageContentEvent(
		evt.MessageID,
		evt.Delta,
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallStart(evt *core.ToolCallStartedEvent) error {
	// First emit TOOL_CALL_AGENT to establish agent-tool call association
	agentEvent := aguievents.NewCustomEvent(
		"TOOL_CALL_AGENT",
		aguievents.WithValue(map[string]interface{}{
			"toolCallId": evt.ToolCallID,
			"agentName":  evt.AgentName,
		}),
	)
	if err := n.emitter.Emit(agentEvent); err != nil {
		return err
	}

	// Then emit standard TOOL_CALL_START event
	uiEvent := aguievents.NewToolCallStartEvent(
		evt.ToolCallID,
		evt.ToolCallName,
		aguievents.WithParentMessageID(evt.ParentMessageID),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallArgs(evt *core.ToolCallArgumentsStreamedEvent) error {
	uiEvent := aguievents.NewToolCallArgsEvent(
		evt.ToolCallID,
		evt.Delta,
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallEnd(evt *core.ToolCallCompletedEvent) error {
	uiEvent := aguievents.NewToolCallEndEvent(
		evt.ToolCallID,
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitTextMessageEnd(evt *core.AssistantMessageCompletedEvent) error {
	uiEvent := aguievents.NewTextMessageEndEvent(
		evt.MessageID,
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallResult(evt *core.ToolCallExecutedEvent) error {
	uiEvent := aguievents.NewToolCallResultEvent(
		evt.MessageID,
		evt.ToolCallID,
		evt.ToolResult,
	)
	return n.emitter.Emit(uiEvent)
}

// Agent collaboration event emitters

func (n *UINotifier) emitMathQuestion(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"MATH_QUESTION",
		aguievents.WithValue(map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitPhysicsQuestion(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"PHYSICS_QUESTION",
		aguievents.WithValue(map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitChemistryQuestion(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"CHEMISTRY_QUESTION",
		aguievents.WithValue(map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitStatisticsQuestion(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"STATISTICS_QUESTION",
		aguievents.WithValue(map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitLinearAlgebraQuestion(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"LINEARALGEBRA_QUESTION",
		aguievents.WithValue(map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitDiagrammingQuestion(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"DIAGRAMMING_QUESTION",
		aguievents.WithValue(map[string]interface{}{
			"request":   evt.GetString("request"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitAgentHandoff(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"AGENT_HANDOFF",
		aguievents.WithValue(map[string]interface{}{
			"fromAgent": evt.GetString("from_agent"),
			"toAgent":   evt.GetString("to_agent"),
			"reason":    evt.GetString("reason"),
			"timestamp": evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitQuestionAnswered(evt *agents.DomainEvent) error {
	uiEvent := aguievents.NewCustomEvent(
		"QUESTION_ANSWERED",
		aguievents.WithValue(map[string]interface{}{
			"subject":    evt.GetString("subject"),
			"answer":     evt.GetString("answer"),
			"answeredBy": evt.GetString("answered_by"),
			"isComplete": evt.GetBool("is_complete"),
			"timestamp":  evt.Timestamp.UnixMilli(),
		}),
	)
	return n.emitter.Emit(uiEvent)
}
