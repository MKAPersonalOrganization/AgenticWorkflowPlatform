package taskmanagement

import (
	"school-with-ai/domain"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// AguiEventEmitter wraps an EventEmitter and converts domain events to AGUI events.
// This adapter allows the orchestration service to remain transport-agnostic while
// the AGUI layer (HTTP/SSE) receives AGUI-specific events.
type AguiEventEmitter struct {
	underlying EventEmitter
}

// NewAguiEventEmitter creates a new AGUI event emitter adapter.
func NewAguiEventEmitter(emitter EventEmitter) *AguiEventEmitter {
	return &AguiEventEmitter{
		underlying: emitter,
	}
}

// Emit converts domain events to AGUI events and emits them.
func (a *AguiEventEmitter) Emit(event any) error {
	// Convert domain events to AGUI events
	switch evt := event.(type) {
	case *domain.RunStartedEvent:
		aguiEvent := aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID)
		return a.underlying.Emit(aguiEvent)

	case *domain.RunFinishedEvent:
		aguiEvent := aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID)
		return a.underlying.Emit(aguiEvent)

	case *domain.RunErrorEvent:
		aguiEvent := aguievents.NewRunErrorEvent(
			evt.Message,
			aguievents.WithErrorCode(evt.ErrorCode),
		)
		return a.underlying.Emit(aguiEvent)

	case *domain.ThreadTitleUpdatedEvent:
		aguiEvent := aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]any{
				"threadId": evt.ThreadID,
				"title":    evt.Title,
			}),
		)
		return a.underlying.Emit(aguiEvent)

	case *domain.AssistantMessageStartedEvent:
		aguiEvent := aguievents.NewTextMessageStartEvent(evt.MessageID, aguievents.WithRole("assistant"))
		return a.underlying.Emit(aguiEvent)

	case *domain.AssistantContentStreamedEvent:
		aguiEvent := aguievents.NewTextMessageContentEvent(evt.MessageID, evt.Delta)
		return a.underlying.Emit(aguiEvent)

	case *domain.AssistantMessageCompletedEvent:
		aguiEvent := aguievents.NewTextMessageEndEvent(evt.MessageID)
		return a.underlying.Emit(aguiEvent)

	case *domain.ToolCallStartedEvent:
		aguiEvent := aguievents.NewToolCallStartEvent(
			evt.ToolCallID,
			evt.ToolCallName,
			aguievents.WithParentMessageID(evt.ParentMessageID),
		)
		return a.underlying.Emit(aguiEvent)

	case *domain.ToolCallArgumentsStreamedEvent:
		aguiEvent := aguievents.NewToolCallArgsEvent(evt.ToolCallID, evt.Delta)
		return a.underlying.Emit(aguiEvent)

	case *domain.ToolCallCompletedEvent:
		aguiEvent := aguievents.NewToolCallEndEvent(evt.ToolCallID)
		return a.underlying.Emit(aguiEvent)

	case *domain.ToolCallExecutedEvent:
		aguiEvent := aguievents.NewToolCallResultEvent(
			evt.MessageID,
			evt.ToolCallID,
			evt.ToolResult,
		)
		return a.underlying.Emit(aguiEvent)

	default:
		// Pass through non-domain events (e.g., AGUI events that were already converted)
		return a.underlying.Emit(event)
	}
}
