package infra

import (
	"net/http"
	"time"

	"school-with-ai/pkg/agui"
	"school-with-ai/services/agent"
)

// SSEEventEmitter implements agent.EventEmitter for Server-Sent Events
type SSEEventEmitter struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

// NewSSEEventEmitter creates a new SSE event emitter
func NewSSEEventEmitter(w http.ResponseWriter, f http.Flusher) agent.EventEmitter {
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
	case *agent.TextMessageStartEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.TextMessageStartEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TEXT_MESSAGE_START",
				Timestamp: &timestamp,
			},
			MessageID: ev.MessageID,
			Role:      ev.Role,
		}

	case *agent.TextMessageContentEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.TextMessageContentEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TEXT_MESSAGE_CONTENT",
				Timestamp: &timestamp,
			},
			MessageID: ev.MessageID,
			Delta:     ev.Delta,
		}

	case *agent.TextMessageEndEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.TextMessageEndEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TEXT_MESSAGE_END",
				Timestamp: &timestamp,
			},
			MessageID: ev.MessageID,
		}

	case *agent.ToolCallStartEvent:
		timestamp := time.Now().UnixMilli()
		parentMessageID := ev.ParentMessageID
		aguiEvent = &agui.ToolCallStartEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_START",
				Timestamp: &timestamp,
			},
			ToolCallID:      ev.ToolCallID,
			ToolCallName:    ev.ToolCallName,
			ParentMessageID: &parentMessageID,
		}

	case *agent.ToolCallArgsEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.ToolCallArgsEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_ARGS",
				Timestamp: &timestamp,
			},
			ToolCallID: ev.ToolCallID,
			Delta:      ev.Delta,
		}

	case *agent.ToolCallEndEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.ToolCallEndEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_END",
				Timestamp: &timestamp,
			},
			ToolCallID: ev.ToolCallID,
		}

	case *agent.ToolCallResultEvent:
		timestamp := time.Now().UnixMilli()
		role := ev.Role
		aguiEvent = &agui.ToolCallResultEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_RESULT",
				Timestamp: &timestamp,
			},
			MessageID:  ev.MessageID,
			ToolCallID: ev.ToolCallID,
			Content:    ev.Content,
			Role:       &role,
		}

	default:
		// If it's already an agui event or unknown type, pass through
		aguiEvent = event
	}

	if err := agui.WriteSSEEvent(e.writer, aguiEvent); err != nil {
		return err
	}
	e.flusher.Flush()
	return nil
}
