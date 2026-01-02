package infra

import (
	"time"

	"school-with-ai/infra/agui"
	"school-with-ai/services/agents"
	"school-with-ai/services/sdk"
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
func (n *UINotifier) HandleDomainEvent(event sdk.Event) error {
	switch evt := event.(type) {
	case *sdk.AssistantMessageStartedEvent:
		return n.emitTextMessageStart(evt)

	case *sdk.AssistantContentStreamedEvent:
		return n.emitTextMessageContent(evt)

	case *sdk.ToolCallStartedEvent:
		return n.emitToolCallStart(evt)

	case *sdk.ToolCallArgumentsStreamedEvent:
		return n.emitToolCallArgs(evt)

	case *sdk.ToolCallCompletedEvent:
		return n.emitToolCallEnd(evt)

	case *sdk.AssistantMessageCompletedEvent:
		return n.emitTextMessageEnd(evt)

	case *sdk.ToolCallExecutedEvent:
		return n.emitToolCallResult(evt)

	// Agent collaboration events
	case *agents.DomainEvent:
		// Handle unified domain events
		switch evt.Type {
		case agents.EventTypeWhiteboardUpdated:
			return n.emitWhiteboardUpdated(evt)
		case agents.EventTypeMathQuestion:
			return n.emitMathQuestion(evt)
		case agents.EventTypePhysicsQuestion:
			return n.emitPhysicsQuestion(evt)
		case agents.EventTypeChemistryQuestion:
			return n.emitChemistryQuestion(evt)
		case agents.EventTypeStatisticsQuestion:
			return n.emitStatisticsQuestion(evt)
		case agents.EventTypeQuestionAnswered:
			return n.emitQuestionAnswered(evt)
		default:
			return nil
		}

	// Other domain events don't need UI events
	default:
		return nil
	}
}

func (n *UINotifier) emitTextMessageStart(evt *sdk.AssistantMessageStartedEvent) error {
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

func (n *UINotifier) emitTextMessageContent(evt *sdk.AssistantContentStreamedEvent) error {
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

func (n *UINotifier) emitToolCallStart(evt *sdk.ToolCallStartedEvent) error {
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

func (n *UINotifier) emitToolCallArgs(evt *sdk.ToolCallArgumentsStreamedEvent) error {
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

func (n *UINotifier) emitToolCallEnd(evt *sdk.ToolCallCompletedEvent) error {
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

func (n *UINotifier) emitTextMessageEnd(evt *sdk.AssistantMessageCompletedEvent) error {
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

func (n *UINotifier) emitToolCallResult(evt *sdk.ToolCallExecutedEvent) error {
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

func (n *UINotifier) emitWhiteboardUpdated(evt *agents.DomainEvent) error {
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

func (n *UINotifier) emitMathQuestion(evt *agents.DomainEvent) error {
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

func (n *UINotifier) emitPhysicsQuestion(evt *agents.DomainEvent) error {
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

func (n *UINotifier) emitChemistryQuestion(evt *agents.DomainEvent) error {
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

func (n *UINotifier) emitStatisticsQuestion(evt *agents.DomainEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "STATISTICS_QUESTION",
		Value: map[string]interface{}{
			"question":  evt.GetString("question"),
			"emittedBy": evt.GetString("emitted_by"),
			"context":   evt.GetString("context"),
			"timestamp": evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitQuestionAnswered(evt *agents.DomainEvent) error {
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
