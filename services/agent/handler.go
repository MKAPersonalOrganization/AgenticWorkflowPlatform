package agent

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/tools"
	"school-with-ai/services/llm"
)

// Repository defines the interface for thread and message storage operations
type Repository interface {
	// GetOrCreateThread retrieves an existing thread or creates a new one
	GetOrCreateThread(threadID string) *domain.Thread

	// AddMessage adds a message to a thread
	AddMessage(threadID string, msg domain.Message)

	// GetMessages retrieves all messages from a thread
	GetMessages(threadID string) []domain.Message

	// GetThread retrieves a thread by ID
	GetThread(threadID string) (*domain.Thread, bool)

	// GetAllThreads retrieves all threads sorted by UpdatedAt (newest first)
	GetAllThreads() []domain.Thread

	// UpdateThreadTitle updates the title of a thread
	UpdateThreadTitle(threadID string, title string)

	// DeleteThread removes a thread from the store
	DeleteThread(threadID string) bool
}

// Agent handles processing of user messages and emits domain events
type Agent struct {
	provider llm.Provider
	store    Repository
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
func NewAgent(provider llm.Provider, store Repository, emitter EventEmitter, opts ...Option) *Agent {
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
	messages []domain.Message,
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
				}
			}
		}
	}

	// Store assistant message and handle tool calls
	if len(activeToolCalls) > 0 {
		// Build tool calls list from active tool calls
		toolCallsList := make([]domain.ToolCall, 0, len(activeToolCalls))
		for _, state := range activeToolCalls {
			toolCallsList = append(toolCallsList, domain.ToolCall{
				ID:   state.ID,
				Type: "function",
				Function: domain.ToolFunction{
					Name:      state.Name,
					Arguments: state.Arguments,
				},
			})
		}

		// Store assistant message with tool calls
		h.store.AddMessage(threadID, domain.Message{
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

			// Execute the tool call immediately
			result, err := h.executeToolCall(toolCall)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}

			// Store tool result message
			toolResultID := GenerateID()
			h.store.AddMessage(threadID, domain.Message{
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
		h.store.AddMessage(threadID, domain.Message{
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
func (h *Agent) executeToolCall(toolCall domain.ToolCall) (string, error) {
	switch toolCall.Function.Name {
	case "get_weather":
		return tools.ExecuteWeatherTool(toolCall.Function.Arguments)
	case "calculate":
		return tools.ExecuteCalculatorTool(toolCall.Function.Arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
	}
}
