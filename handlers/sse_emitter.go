package handlers

import (
	"net/http"

	"school-with-ai/internal/agui"
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

// Emit writes an event as SSE and flushes the response
func (e *SSEEventEmitter) Emit(event any) error {
	if err := agui.WriteSSEEvent(e.writer, event); err != nil {
		return err
	}
	e.flusher.Flush()
	return nil
}
