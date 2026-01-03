package infra

import (
	"encoding/json"
	"fmt"
	"net/http"

	"school-with-ai/services/sdk"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// writeSSEEvent writes an event to the response writer in SSE format
func writeSSEEvent(w http.ResponseWriter, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// SSEEventEmitter implements sdk.EventEmitter for Server-Sent Events
type SSEEventEmitter struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

// NewSSEEventEmitter creates a new SSE event emitter
func NewSSEEventEmitter(w http.ResponseWriter, f http.Flusher) EventEmitter {
	return &SSEEventEmitter{
		writer:  w,
		flusher: f,
	}
}

// Emit translates domain events to agui events and writes them as SSE
func (e *SSEEventEmitter) Emit(event any) error {
	var aguiEvent any

	// Translate domain events to agui events
	switch ev := event.(type) {
	case *sdk.TextMessageStartEvent:
		role := ev.Role
		aguiEvent = aguievents.NewTextMessageStartEvent(
			ev.MessageID,
			aguievents.WithRole(role),
		)

	case *sdk.TextMessageContentEvent:
		aguiEvent = aguievents.NewTextMessageContentEvent(
			ev.MessageID,
			ev.Delta,
		)

	case *sdk.TextMessageEndEvent:
		aguiEvent = aguievents.NewTextMessageEndEvent(
			ev.MessageID,
		)

	case *sdk.ToolCallStartEvent:
		aguiEvent = aguievents.NewToolCallStartEvent(
			ev.ToolCallID,
			ev.ToolCallName,
			aguievents.WithParentMessageID(ev.ParentMessageID),
		)

	case *sdk.ToolCallArgsEvent:
		aguiEvent = aguievents.NewToolCallArgsEvent(
			ev.ToolCallID,
			ev.Delta,
		)

	case *sdk.ToolCallEndEvent:
		aguiEvent = aguievents.NewToolCallEndEvent(
			ev.ToolCallID,
		)

	case *sdk.ToolCallResultEvent:
		aguiEvent = aguievents.NewToolCallResultEvent(
			ev.MessageID,
			ev.ToolCallID,
			ev.Content,
		)

	default:
		// If it's already an agui event or unknown type, pass through
		aguiEvent = event
	}

	if err := writeSSEEvent(e.writer, aguiEvent); err != nil {
		return err
	}
	e.flusher.Flush()
	return nil
}
