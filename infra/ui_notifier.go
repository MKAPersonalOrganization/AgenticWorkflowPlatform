package infra

import (
	"time"

	"school-with-ai/infra/agui"
	"school-with-ai/services/agent"
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
func (n *UINotifier) HandleDomainEvent(event agent.Event) error {
	switch evt := event.(type) {
	case *agent.AssistantMessageStartedEvent:
		return n.emitTextMessageStart(evt)

	case *agent.AssistantContentStreamedEvent:
		return n.emitTextMessageContent(evt)

	case *agent.ToolCallStartedEvent:
		return n.emitToolCallStart(evt)

	case *agent.ToolCallArgumentsStreamedEvent:
		return n.emitToolCallArgs(evt)

	case *agent.ToolCallCompletedEvent:
		return n.emitToolCallEnd(evt)

	case *agent.AssistantMessageCompletedEvent:
		return n.emitTextMessageEnd(evt)

	case *agent.ToolCallExecutedEvent:
		return n.emitToolCallResult(evt)

	// Agent collaboration events
	case *agent.DomainEvent:
		// Handle unified domain events
		switch evt.Type {
		case agent.EventTypeAgentCalled:
			return n.emitAgentCalled(evt)
		case agent.EventTypeWhiteboardUpdated:
			return n.emitWhiteboardUpdated(evt)
		case agent.EventTypeMathQuestion:
			return n.emitMathQuestion(evt)
		case agent.EventTypePhysicsQuestion:
			return n.emitPhysicsQuestion(evt)
		case agent.EventTypeChemistryQuestion:
			return n.emitChemistryQuestion(evt)
		case agent.EventTypeQuestionAnswered:
			return n.emitQuestionAnswered(evt)
		default:
			return nil
		}

	// Other domain events don't need UI events
	default:
		return nil
	}
}

func (n *UINotifier) emitTextMessageStart(evt *agent.AssistantMessageStartedEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.TextMessageStartEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "TEXT_MESSAGE_START",
			Timestamp: &timestamp,
		},
		MessageID: evt.MessageID,
		Role:      "assistant",
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitTextMessageContent(evt *agent.AssistantContentStreamedEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.TextMessageContentEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "TEXT_MESSAGE_CONTENT",
			Timestamp: &timestamp,
		},
		MessageID: evt.MessageID,
		Delta:     evt.Delta,
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallStart(evt *agent.ToolCallStartedEvent) error {
	timestamp := time.Now().UnixMilli()
	parentMessageID := evt.ParentMessageID
	uiEvent := &agui.ToolCallStartEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "TOOL_CALL_START",
			Timestamp: &timestamp,
		},
		ToolCallID:      evt.ToolCallID,
		ToolCallName:    evt.ToolCallName,
		ParentMessageID: &parentMessageID,
		AgentName:       evt.AgentName,
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallArgs(evt *agent.ToolCallArgumentsStreamedEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.ToolCallArgsEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "TOOL_CALL_ARGS",
			Timestamp: &timestamp,
		},
		ToolCallID: evt.ToolCallID,
		Delta:      evt.Delta,
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallEnd(evt *agent.ToolCallCompletedEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.ToolCallEndEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "TOOL_CALL_END",
			Timestamp: &timestamp,
		},
		ToolCallID: evt.ToolCallID,
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitTextMessageEnd(evt *agent.AssistantMessageCompletedEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.TextMessageEndEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "TEXT_MESSAGE_END",
			Timestamp: &timestamp,
		},
		MessageID: evt.MessageID,
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitToolCallResult(evt *agent.ToolCallExecutedEvent) error {
	timestamp := time.Now().UnixMilli()
	role := "tool"
	uiEvent := &agui.ToolCallResultEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "TOOL_CALL_RESULT",
			Timestamp: &timestamp,
		},
		MessageID:  evt.MessageID,
		ToolCallID: evt.ToolCallID,
		Content:    evt.ToolResult,
		Role:       &role,
	}
	return n.emitter.Emit(uiEvent)
}

// Agent collaboration event emitters

func (n *UINotifier) emitAgentCalled(evt *agent.DomainEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "AGENT_CALLED",
		Value: map[string]interface{}{
			"callerAgent":     evt.GetString("caller_agent"),
			"calledAgent":     evt.GetString("called_agent"),
			"reason":          evt.GetString("reason"),
			"context":         evt.GetString("context"),
			"whiteboardState": evt.GetString("whiteboard_state"),
			"timestamp":       evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitWhiteboardUpdated(evt *agent.DomainEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "WHITEBOARD_UPDATED",
		Value: map[string]interface{}{
			"agentName": evt.GetString("agent_name"),
			"content":   evt.GetString("content"),
			"action":    evt.GetString("action"),
			"timestamp": evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitMathQuestion(evt *agent.DomainEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "MATH_QUESTION",
		Value: map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitPhysicsQuestion(evt *agent.DomainEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "PHYSICS_QUESTION",
		Value: map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitChemistryQuestion(evt *agent.DomainEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "CHEMISTRY_QUESTION",
		Value: map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitQuestionAnswered(evt *agent.DomainEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "QUESTION_ANSWERED",
		Value: map[string]interface{}{
			"subject":    evt.GetString("subject"),
			"answer":     evt.GetString("answer"),
			"answeredBy": evt.GetString("answered_by"),
			"isComplete": evt.GetBool("is_complete"),
			"timestamp":  evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}
