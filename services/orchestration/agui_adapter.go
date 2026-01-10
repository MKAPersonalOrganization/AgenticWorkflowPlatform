package orchestration

import (
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
// Domain events from the orchestration service are converted to AGUI protocol events.
// Other events (like those from UINotifier) are passed through unchanged.
func (a *AguiEventEmitter) Emit(event any) error {
	// Convert domain events to AGUI events
	switch evt := event.(type) {
	case *RunStartedEvent:
		aguiEvent := aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID)
		return a.underlying.Emit(aguiEvent)

	case *RunFinishedEvent:
		aguiEvent := aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID)
		return a.underlying.Emit(aguiEvent)

	case *RunErrorEvent:
		aguiEvent := aguievents.NewRunErrorEvent(
			evt.Message,
			aguievents.WithErrorCode(evt.ErrorCode),
		)
		return a.underlying.Emit(aguiEvent)

	case *ThreadTitleUpdatedEvent:
		aguiEvent := aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]string{
				"threadId": evt.ThreadID,
				"title":    evt.Title,
			}),
		)
		return a.underlying.Emit(aguiEvent)

	default:
		// Pass through non-domain events (e.g., AGUI events from UINotifier)
		return a.underlying.Emit(event)
	}
}
