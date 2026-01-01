package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/llm"
	"school-with-ai/services/tools"
)

// Agent handles processing of user messages using pure event sourcing
// It ONLY emits domain events - no UI concerns
type Agent struct {
	config   AgentConfig
	provider Provider
	log      EventLog
	state    ThreadState
}

// Option is a functional option for configuring Agent
type Option func(*Agent)

// WithTools configures the agent with available tools
func WithTools(tools []llm.Tool) Option {
	return func(h *Agent) {
		h.config.Tools = tools
	}
}

// WithConfig configures the agent with a complete configuration
func WithConfig(config AgentConfig) Option {
	return func(h *Agent) {
		h.config = config
	}
}

// NewAgent creates a new agent with event sourcing (no UI dependencies)
func NewAgent(provider Provider, log EventLog, state ThreadState, opts ...Option) *Agent {
	h := &Agent{
		provider: provider,
		log:      log,
		state:    state,
		config:   DefaultConfig("agent"), // Default config
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	return h
}

// DomainEventHandler is called for each domain event emitted by the agent
// This allows external systems to react to events (e.g., UI projections, logging, etc.)
type DomainEventHandler func(event Event) error

// Run processes the LLM streaming response and emits domain events
// eventHandler is called for each domain event (for UI projections, etc.)
func (h *Agent) Run(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []domain.Message,
	eventHandler DomainEventHandler,
) error {
	return h.runIteration(ctx, threadID, messageID, messages, eventHandler)
}

// runIteration processes a single LLM iteration and recursively continues if tools are called
func (h *Agent) runIteration(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []domain.Message,
	eventHandler DomainEventHandler,
) error {
	// Check if client disconnected before starting LLM call
	select {
	case <-ctx.Done():
		return fmt.Errorf("client disconnected: %w", ctx.Err())
	default:
	}

	// Emit AssistantMessageStartedEvent
	startEvent := &AssistantMessageStartedEvent{
		BaseEvent: BaseEvent{
			Type:      "AssistantMessageStarted",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
	}
	if err := h.emitDomainEvent(startEvent, eventHandler); err != nil {
		return err
	}

	// Create streaming completion with agent's system prompt
	config := llm.CompletionConfig{
		Messages:     messages,
		Tools:        h.config.Tools,
		SystemPrompt: h.config.SystemPrompt,
	}

	streamIter := h.provider.CreateStreamingCompletion(ctx, config)

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
		// Check if client disconnected during streaming
		select {
		case <-ctx.Done():
			// Save partial content as event before returning
			if completeContent != "" {
				evt := &AssistantMessageCompletedEvent{
					BaseEvent: BaseEvent{
						Type:      "AssistantMessageCompleted",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					MessageID: messageID,
					Content:   completeContent,
					AgentName: h.config.Name,
				}
				h.emitDomainEvent(evt, eventHandler)
			}
			return fmt.Errorf("client disconnected during streaming: %w", ctx.Err())
		default:
		}

		// Check for errors
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Handle text content - emit streaming event for each delta
		if chunk.Content != "" {
			completeContent += chunk.Content

			// Emit AssistantContentStreamedEvent for each delta
			contentEvent := &AssistantContentStreamedEvent{
				BaseEvent: BaseEvent{
					Type:      "AssistantContentStreamed",
					Timestamp: time.Now(),
					ThreadID:  threadID,
				},
				MessageID: messageID,
				Delta:     chunk.Content,
			}
			if err := h.emitDomainEvent(contentEvent, eventHandler); err != nil {
				return err
			}
		}

		// Handle tool calls
		if len(chunk.ToolCalls) > 0 {
			for _, toolCallDelta := range chunk.ToolCalls {
				index := toolCallDelta.Index
				var state *toolCallState

				// Check if this is a new tool call
				if toolCallDelta.ID != "" && toolCallDelta.Name != "" {
					// Emit ToolCallStartedEvent
					toolStartEvent := &ToolCallStartedEvent{
						BaseEvent: BaseEvent{
							Type:      "ToolCallStarted",
							Timestamp: time.Now(),
							ThreadID:  threadID,
						},
						ToolCallID:      toolCallDelta.ID,
						ToolCallName:    toolCallDelta.Name,
						ParentMessageID: messageID,
						AgentName:       h.config.Name,
					}
					if err := h.emitDomainEvent(toolStartEvent, eventHandler); err != nil {
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

				// Emit streaming event for arguments
				if state != nil && toolCallDelta.Arguments != "" {
					argsEvent := &ToolCallArgumentsStreamedEvent{
						BaseEvent: BaseEvent{
							Type:      "ToolCallArgumentsStreamed",
							Timestamp: time.Now(),
							ThreadID:  threadID,
						},
						ToolCallID: state.ID,
						Delta:      toolCallDelta.Arguments,
					}
					if err := h.emitDomainEvent(argsEvent, eventHandler); err != nil {
						return err
					}

					state.Arguments += toolCallDelta.Arguments
				}
			}
		}
	}

	// Emit ToolCallCompletedEvent for each tool call
	for _, state := range activeToolCalls {
		completedEvent := &ToolCallCompletedEvent{
			BaseEvent: BaseEvent{
				Type:      "ToolCallCompleted",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			ToolCallID: state.ID,
		}
		if err := h.emitDomainEvent(completedEvent, eventHandler); err != nil {
			return err
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

		// Emit AssistantMessageCompletedEvent with tool calls
		assistantEvent := &AssistantMessageCompletedEvent{
			BaseEvent: BaseEvent{
				Type:      "AssistantMessageCompleted",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			MessageID: messageID,
			Content:   completeContent,
			ToolCalls: toolCallsList,
			AgentName: h.config.Name,
		}
		if err := h.emitDomainEvent(assistantEvent, eventHandler); err != nil {
			return err
		}

		// For each tool call: execute and emit result
		for _, toolCall := range toolCallsList {
			// Execute the tool call
			result, err := h.executeToolCall(threadID, toolCall, eventHandler)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}

			// Emit ToolCallExecutedEvent
			toolResultID := GenerateID()
			toolEvent := &ToolCallExecutedEvent{
				BaseEvent: BaseEvent{
					Type:      "ToolCallStarted",
					Timestamp: time.Now(),
					ThreadID:  threadID,
				},
				MessageID:       toolResultID,
				ToolCallID:      toolCall.ID,
				ToolName:        toolCall.Function.Name,
				ToolArguments:   toolCall.Function.Arguments,
				ToolResult:      result,
				ParentMessageID: messageID,
				AgentName:       h.config.Name,
			}
			if err := h.emitDomainEvent(toolEvent, eventHandler); err != nil {
				return err
			}
		}

		// Automatically continue conversation with tool results
		// Exception: If agent called another agent, stop iteration
		// The worker pool will handle switching to the called agent
		for _, toolCall := range toolCallsList {
			if toolCall.Function.Name == "call_agent" {
				// Agent's job is done - don't continue iteration
				return nil
			}
		}

		// Get updated messages including tool results from state
		updatedMessages := h.state.Messages(threadID)

		// Generate new message ID for the continuation
		continuationMessageID := GenerateID()

		// Recursively call to process the LLM response with tool results
		return h.runIteration(ctx, threadID, continuationMessageID, updatedMessages, eventHandler)
	}

	// No tool calls, just emit the completed message event
	assistantEvent := &AssistantMessageCompletedEvent{
		BaseEvent: BaseEvent{
			Type:      "AssistantMessageCompleted",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
		Content:   completeContent,
		AgentName: h.config.Name,
	}
	if err := h.emitDomainEvent(assistantEvent, eventHandler); err != nil {
		return err
	}

	return nil
}

// emitDomainEvent persists the event and notifies handlers
func (h *Agent) emitDomainEvent(event Event, eventHandler DomainEventHandler) error {
	// Append to event log
	if err := h.log.Append(event.AggregateID(), event); err != nil {
		return err
	}

	// Update thread state
	if err := h.state.Update(event); err != nil {
		return err
	}

	// Notify external handler (e.g., UI projection)
	if eventHandler != nil {
		if err := eventHandler(event); err != nil {
			return err
		}
	}

	return nil
}

// executeToolCall executes a tool call and returns the result
// For instructor tools, it also emits domain events
func (h *Agent) executeToolCall(threadID string, toolCall domain.ToolCall, eventHandler DomainEventHandler) (string, error) {
	toolName := toolCall.Function.Name

	// Handle agent collaboration tools - they emit domain events
	switch toolName {
	case "call_agent":
		// Parse the arguments to extract agent call info
		var input struct {
			AgentName string `json:"agent_name"`
			Reason    string `json:"reason"`
			Context   string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return "", fmt.Errorf("failed to parse call_agent arguments: %w", err)
		}

		// Get current whiteboard state from thread
		thread, exists := h.state.Get(threadID)
		whiteboardState := ""
		if exists {
			whiteboardState = thread.WhiteboardContent
		}

		// Emit AgentCalledEvent
		agentCalledEvent := NewAgentCalled(
			threadID,
			h.config.Name,
			input.AgentName,
			input.Reason,
			input.Context,
			whiteboardState,
		)
		if err := h.emitDomainEvent(agentCalledEvent, eventHandler); err != nil {
			return "", err
		}

		return tools.ExecuteInstructorTool(toolName, toolCall.Function.Arguments)

	case "write_whiteboard":
		// Parse the arguments to extract whiteboard info
		var input struct {
			Content string `json:"content"`
			Action  string `json:"action"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return "", fmt.Errorf("failed to parse write_whiteboard arguments: %w", err)
		}

		// Emit WhiteboardUpdatedEvent
		whiteboardEvent := NewWhiteboardUpdated(threadID, h.config.Name, input.Content, input.Action)
		if err := h.emitDomainEvent(whiteboardEvent, eventHandler); err != nil {
			return "", err
		}

		return tools.ExecuteInstructorTool(toolName, toolCall.Function.Arguments)

	case "emit_event":
		// Parse the arguments to extract event info
		var input struct {
			EventName   string                 `json:"event_name"`
			Payload     map[string]interface{} `json:"payload"`
			Description string                 `json:"description"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return "", fmt.Errorf("failed to parse emit_event arguments: %w", err)
		}

		// Emit CustomEventEmittedEvent
		customEvent := NewCustomEventEmitted(threadID, input.EventName, h.config.Name, input.Description, input.Payload)
		if err := h.emitDomainEvent(customEvent, eventHandler); err != nil {
			return "", err
		}

		return tools.ExecuteInstructorTool(toolName, toolCall.Function.Arguments)

	// Regular tools
	case "get_weather":
		return tools.ExecuteWeatherTool(toolCall.Function.Arguments)
	case "calculate":
		return tools.ExecuteCalculatorTool(toolCall.Function.Arguments)
	default:
		// Try instructor tools as fallback
		return tools.ExecuteInstructorTool(toolName, toolCall.Function.Arguments)
	}
}
