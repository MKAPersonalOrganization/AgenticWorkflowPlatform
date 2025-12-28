package agent

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/internal/session"
	"school-with-ai/internal/tools"
	"school-with-ai/services/llm"
)

// Agent handles processing of user messages and emits domain events
type Agent struct {
	provider llm.Provider
	store    *session.Store
	emitter  EventEmitter
	tools    []llm.Tool
}

// Option is a functional option for configuring Agent
type Option func(*Agent)

// WithTools configures the handler with available tools
func WithTools(tools []llm.Tool) Option {
	return func(h *Agent) {
		h.tools = tools
	}
}

// NewAgent creates a new agent
func NewAgent(provider llm.Provider, store *session.Store, emitter EventEmitter, opts ...Option) *Agent {
	h := &Agent{
		provider: provider,
		store:    store,
		emitter:  emitter,
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Run processes the LLM streaming response and emits AG-UI events
func (h *Agent) Run(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []session.Message,
) error {
	// Create streaming completion
	config := llm.CompletionConfig{
		Messages: messages,
		Tools:    h.tools,
	}

	streamIter := h.provider.CreateStreamingCompletion(ctx, config)

	// Send TEXT_MESSAGE_START event
	startEvent := &TextMessageStartEvent{
		MessageID: messageID,
		Role:      "assistant",
	}
	if err := h.emitter.Emit(startEvent); err != nil {
		return err
	}

	// Track complete content for storage
	var completeContent string

	// Track active tool calls
	type toolCallState struct {
		ID        string
		Index     int
		Name      string
		Arguments string
	}
	activeToolCalls := make(map[string]*toolCallState)
	toolCallsByIndex := make(map[int]*toolCallState)

	// Process stream chunks using iterator
	for chunk, err := range streamIter {
		// Check for errors
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Handle text content
		if chunk.Content != "" {
			completeContent += chunk.Content

			// Send TEXT_MESSAGE_CONTENT event
			contentEvent := &TextMessageContentEvent{
				MessageID: messageID,
				Delta:     chunk.Content,
			}
			if err := h.emitter.Emit(contentEvent); err != nil {
				return err
			}
		}

		// Handle tool calls
		if len(chunk.ToolCalls) > 0 {
			for _, toolCallDelta := range chunk.ToolCalls {
				index := toolCallDelta.Index
				var state *toolCallState

				// Check if this is a new tool call (has ID and function name)
				if toolCallDelta.ID != "" && toolCallDelta.Name != "" {
					// Emit TOOL_CALL_START
					toolStartEvent := &ToolCallStartEvent{
						ToolCallID:      toolCallDelta.ID,
						ToolCallName:    toolCallDelta.Name,
						ParentMessageID: messageID,
					}
					if err := h.emitter.Emit(toolStartEvent); err != nil {
						return err
					}

					// Track this tool call
					state = &toolCallState{
						ID:        toolCallDelta.ID,
						Index:     index,
						Name:      toolCallDelta.Name,
						Arguments: "",
					}
					activeToolCalls[toolCallDelta.ID] = state
					toolCallsByIndex[index] = state
				} else {
					// This is a continuation - find by index
					state = toolCallsByIndex[index]
					if state == nil {
						fmt.Printf("DEBUG: WARNING - No state found for tool call index %d\n", index)
					}
				}

				// Stream function arguments if present
				if state != nil && toolCallDelta.Arguments != "" {
					// Emit TOOL_CALL_ARGS event
					argsEvent := &ToolCallArgsEvent{
						ToolCallID: state.ID,
						Delta:      toolCallDelta.Arguments,
					}
					if err := h.emitter.Emit(argsEvent); err != nil {
						return err
					}

					// Accumulate arguments
					state.Arguments += toolCallDelta.Arguments
					fmt.Printf("DEBUG: Accumulated args for %s (index %d): delta='%s', total='%s'\n",
						state.ID, state.Index, toolCallDelta.Arguments, state.Arguments)
				} else if state != nil {
					fmt.Printf("DEBUG: Skipping empty delta for tool call %s (index %d)\n", state.ID, state.Index)
				}
			}
		}
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

		// For each tool call: emit END, execute, emit RESULT immediately
		for _, toolCall := range toolCallsList {
			// Emit TOOL_CALL_END
			endEvent := &ToolCallEndEvent{
				ToolCallID: toolCall.ID,
			}
			if err := h.emitter.Emit(endEvent); err != nil {
				return err
			}

			// Debug: log tool call details
			fmt.Printf("DEBUG: Executing tool call %s - Name: %s, Args length: %d, Args: %s\n",
				toolCall.ID, toolCall.Function.Name, len(toolCall.Function.Arguments), toolCall.Function.Arguments)

			// Execute the tool call immediately
			result, err := h.executeToolCall(toolCall)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}

			// Store tool result message
			toolResultID := GenerateID()
			h.store.AddMessage(threadID, session.Message{
				ID:         toolResultID,
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
				Timestamp:  time.Now(),
			})

			// Emit TOOL_CALL_RESULT event immediately after execution
			resultEvent := &ToolCallResultEvent{
				MessageID:  toolResultID,
				ToolCallID: toolCall.ID,
				Content:    result,
				Role:       "tool",
			}
			if err := h.emitter.Emit(resultEvent); err != nil {
				return err
			}
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
	endEvent := &TextMessageEndEvent{
		MessageID: messageID,
	}
	if err := h.emitter.Emit(endEvent); err != nil {
		return err
	}

	return nil
}

// executeToolCall executes a tool call and returns the result
func (h *Agent) executeToolCall(toolCall session.ToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "get_weather":
		return tools.ExecuteWeatherTool(toolCall.Function.Arguments)
	case "calculate":
		return tools.ExecuteCalculatorTool(toolCall.Function.Arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}
