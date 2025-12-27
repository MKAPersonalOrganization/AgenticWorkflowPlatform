package openai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/openai/openai-go/v3"
	"school-with-ai/internal/agui"
	"school-with-ai/internal/session"
	"school-with-ai/internal/tools"
)

// StreamHandler handles streaming OpenAI responses and emits AG-UI events
type StreamHandler struct {
	client  *Client
	store   *session.Store
	flusher http.Flusher
	writer  http.ResponseWriter
}

// ToolCallState tracks streaming tool call data
type ToolCallState struct {
	ID        string
	Index     int
	Name      string
	Arguments string
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
func (h *StreamHandler) HandleStream(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []session.Message,
	tools []openai.ChatCompletionToolUnionParam,
) error {
	// Create streaming completion with tools
	stream := h.client.CreateStreamingCompletion(ctx, messages, tools)

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

	// Track active tool calls
	activeToolCalls := make(map[string]*ToolCallState)
	toolCallsByIndex := make(map[int]*ToolCallState)

	// Process stream chunks
	for stream.Next() {
		chunk := stream.Current()

		// Extract content from the first choice
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta

			// Handle text content
			if delta.Content != "" {
				completeContent += delta.Content

				// Send TEXT_MESSAGE_CONTENT event
				contentEvent := &agui.TextMessageContentEvent{
					Type:      "TEXT_MESSAGE_CONTENT",
					MessageID: messageID,
					Delta:     delta.Content,
					Timestamp: time.Now().UnixMilli(),
				}
				if err := agui.WriteSSEEvent(h.writer, contentEvent); err != nil {
					return err
				}
				h.flusher.Flush()
			}

			// Handle tool calls
			if len(delta.ToolCalls) > 0 {
				for _, toolCallDelta := range delta.ToolCalls {
					index := int(toolCallDelta.Index)
					var state *ToolCallState

					// Check if this is a new tool call (has ID and function name)
					if toolCallDelta.ID != "" && toolCallDelta.Function.Name != "" {
						// Emit TOOL_CALL_START
						toolStartEvent := &agui.ToolCallStartEvent{
							Type:            "TOOL_CALL_START",
							ToolCallID:      toolCallDelta.ID,
							ToolCallName:    toolCallDelta.Function.Name,
							ParentMessageID: messageID,
							Timestamp:       time.Now().UnixMilli(),
						}
						if err := agui.WriteSSEEvent(h.writer, toolStartEvent); err != nil {
							return err
						}
						h.flusher.Flush()

						// Track this tool call
						state = &ToolCallState{
							ID:        toolCallDelta.ID,
							Index:     index,
							Name:      toolCallDelta.Function.Name,
							Arguments: "",
						}
						activeToolCalls[toolCallDelta.ID] = state
						toolCallsByIndex[index] = state
					} else {
						// This is a continuation - find by index
						state = toolCallsByIndex[index]
					}

					// Stream function arguments if present
					if state != nil && toolCallDelta.Function.Arguments != "" {
						// Emit TOOL_CALL_ARGS event
						argsEvent := &agui.ToolCallArgsEvent{
							Type:       "TOOL_CALL_ARGS",
							ToolCallID: state.ID,
							Delta:      toolCallDelta.Function.Arguments,
							Timestamp:  time.Now().UnixMilli(),
						}
						if err := agui.WriteSSEEvent(h.writer, argsEvent); err != nil {
							return err
						}
						h.flusher.Flush()

						// Accumulate arguments
						state.Arguments += toolCallDelta.Function.Arguments
					}
				}
			}
		}
	}

	// Check for stream errors
	if err := stream.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	// Emit TOOL_CALL_END for all active tool calls
	for toolCallID := range activeToolCalls {
		endEvent := &agui.ToolCallEndEvent{
			Type:       "TOOL_CALL_END",
			ToolCallID: toolCallID,
			Timestamp:  time.Now().UnixMilli(),
		}
		if err := agui.WriteSSEEvent(h.writer, endEvent); err != nil {
			return err
		}
		h.flusher.Flush()
	}

	// Store assistant message and handle tool calls
	if len(activeToolCalls) > 0 {
		// Build tool calls list from active tool calls
		toolCallsList := make([]session.ToolCall, 0, len(activeToolCalls))
		for _, state := range activeToolCalls {
			toolCallsList = append(toolCallsList, session.ToolCall{
				ID:   state.ID,
				Type: "function",
				Function: session.ToolFunction{
					Name:      state.Name,
					Arguments: state.Arguments,
				},
			})
		}

		// Store assistant message with tool calls
		h.store.AddMessage(threadID, session.Message{
			ID:        messageID,
			Role:      "assistant",
			Content:   completeContent,
			ToolCalls: toolCallsList,
			Timestamp: time.Now(),
		})

		// Execute each tool call and emit results
		for _, toolCall := range toolCallsList {
			result, err := h.executeToolCall(toolCall)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}

			// Store tool result message
			toolResultID := agui.GenerateMessageID()
			h.store.AddMessage(threadID, session.Message{
				ID:         toolResultID,
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
				Timestamp:  time.Now(),
			})

			// Emit TOOL_CALL_RESULT event
			resultEvent := &agui.ToolCallResultEvent{
				Type:       "TOOL_CALL_RESULT",
				MessageID:  toolResultID,
				ToolCallID: toolCall.ID,
				Content:    result,
				Role:       "tool",
				Timestamp:  time.Now().UnixMilli(),
			}
			if err := agui.WriteSSEEvent(h.writer, resultEvent); err != nil {
				return err
			}
			h.flusher.Flush()
		}
	} else {
		// No tool calls, just store the text message
		h.store.AddMessage(threadID, session.Message{
			ID:        messageID,
			Role:      "assistant",
			Content:   completeContent,
			Timestamp: time.Now(),
		})
	}

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

// executeToolCall executes a tool call and returns the result
func (h *StreamHandler) executeToolCall(toolCall session.ToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "get_weather":
		return tools.ExecuteWeatherTool(toolCall.Function.Arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}
