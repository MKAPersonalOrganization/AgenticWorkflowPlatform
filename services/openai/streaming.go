package openai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"school-with-ai/internal/agui"
	"school-with-ai/internal/session"
)

// StreamHandler handles streaming OpenAI responses and emits AG-UI events
type StreamHandler struct {
	client  *Client
	store   *session.Store
	flusher http.Flusher
	writer  http.ResponseWriter
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(client *Client, store *session.Store, w http.ResponseWriter, f http.Flusher) *StreamHandler {
	return &StreamHandler{
		client:  client,
		store:   store,
		flusher: f,
		writer:  w,
	}
}

// HandleStream processes the OpenAI streaming response and emits AG-UI events
func (h *StreamHandler) HandleStream(ctx context.Context, threadID string, messageID string, messages []session.Message) error {
	// Create streaming completion
	stream := h.client.CreateStreamingCompletion(ctx, messages)

	// Send TEXT_MESSAGE_START event
	startEvent := &agui.TextMessageStartEvent{
		Type:      "TEXT_MESSAGE_START",
		MessageID: messageID,
		Role:      "assistant",
		Timestamp: time.Now().UnixMilli(),
	}
	if err := agui.WriteSSEEvent(h.writer, startEvent); err != nil {
		return err
	}
	h.flusher.Flush()

	// Track complete content for storage
	var completeContent string

	// Process stream chunks
	for stream.Next() {
		chunk := stream.Current()

		// Extract content from the first choice
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content

			if delta != "" {
				completeContent += delta

				// Send TEXT_MESSAGE_CONTENT event
				contentEvent := &agui.TextMessageContentEvent{
					Type:      "TEXT_MESSAGE_CONTENT",
					MessageID: messageID,
					Delta:     delta,
					Timestamp: time.Now().UnixMilli(),
				}
				if err := agui.WriteSSEEvent(h.writer, contentEvent); err != nil {
					return err
				}
				h.flusher.Flush()
			}
		}
	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	// Store assistant message
	h.store.AddMessage(threadID, session.Message{
		ID:        messageID,
		Role:      "assistant",
		Content:   completeContent,
		Timestamp: time.Now(),
	})

	// Send TEXT_MESSAGE_END event
	endEvent := &agui.TextMessageEndEvent{
		Type:      "TEXT_MESSAGE_END",
		MessageID: messageID,
		Timestamp: time.Now().UnixMilli(),
	}
	if err := agui.WriteSSEEvent(h.writer, endEvent); err != nil {
		return err
	}
	h.flusher.Flush()

	return nil
}
