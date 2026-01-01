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
	case *agent.AgentCalledEvent:
		return n.emitAgentCalled(evt)

	case *agent.WhiteboardUpdatedEvent:
		return n.emitWhiteboardUpdated(evt)

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

func (n *UINotifier) emitAgentCalled(evt *agent.AgentCalledEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "AGENT_CALLED",
		Value: map[string]interface{}{
			"callerAgent":     evt.CallerAgent,
			"calledAgent":     evt.CalledAgent,
			"reason":          evt.Reason,
			"context":         evt.Context,
			"whiteboardState": evt.WhiteboardState,
			"timestamp":       evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}

func (n *UINotifier) emitWhiteboardUpdated(evt *agent.WhiteboardUpdatedEvent) error {
	timestamp := time.Now().UnixMilli()
	uiEvent := &agui.CustomEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "CUSTOM",
			Timestamp: &timestamp,
		},
		Name: "WHITEBOARD_UPDATED",
		Value: map[string]interface{}{
			"agentName": evt.AgentName,
			"content":   evt.Content,
			"action":    evt.Action,
			"timestamp": evt.Timestamp.UnixMilli(),
		},
	}
	return n.emitter.Emit(uiEvent)
}
