package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/llm"
)

// Agent handles processing of user messages using pure event sourcing
// It ONLY emits domain events - no UI concerns
type Agent struct {
	config       AgentConfig
	provider     Provider
	log          EventLog
	state        ThreadState
	toolRegistry *ToolRegistry
	toolExecutor ToolExecutor
}

// Option is a functional option for configuring Agent
type Option func(*Agent)

// WithTools configures the agent with available tools
func WithTools(tools []Tool) Option {
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

// WithToolExecutor configures the agent with a custom tool executor
func WithToolExecutor(executor ToolExecutor) Option {
	return func(h *Agent) {
		h.toolExecutor = executor
	}
}

// NewAgent creates a new agent with event sourcing (no UI dependencies)
func NewAgent(provider Provider, log EventLog, state ThreadState, opts ...Option) *Agent {
	// Create tool registry
	toolRegistry := NewToolRegistry()

	h := &Agent{
		provider:     provider,
		log:          log,
		state:        state,
		config:       DefaultConfig("agent"),             // Default config
		toolRegistry: toolRegistry,                       // Tool registry
		toolExecutor: NewBasicToolExecutor(toolRegistry), // Default tool executor
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	// Register configured tools in the registry
	for _, tool := range h.config.Tools {
		toolRegistry.Register(tool)
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
		Tools:        ToLLMTools(h.config.Tools),
		SystemPrompt: h.config.SystemPrompt,
	}

	streamIter := h.provider.CreateStreamingCompletion(ctx, config)

	// Track complete content for storage
	var completeContent string

	// Use accumulator to track streaming tool calls
	toolCalls := NewToolCallAccumulator()

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
				_ = h.emitDomainEvent(evt, eventHandler)
			}
			return fmt.Errorf("client disconnected during streaming: %w", ctx.Err())
		default:
		} // Check for errors
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Handle text content - emit streaming event for each delta
		if chunk.Content != "" {
			completeContent += chunk.Content

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

		// Handle tool calls using accumulator
		for _, delta := range chunk.ToolCalls {
			update, ok := toolCalls.ProcessDelta(delta)
			if !ok {
				continue
			}

			if update.IsNew {
				// Emit ToolCallStartedEvent for new tool calls
				toolStartEvent := &ToolCallStartedEvent{
					BaseEvent: BaseEvent{
						Type:      "ToolCallStarted",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID:      update.ID,
					ToolCallName:    update.Name,
					ParentMessageID: messageID,
					AgentName:       h.config.Name,
				}
				if err := h.emitDomainEvent(toolStartEvent, eventHandler); err != nil {
					return err
				}
			}

			// Emit streaming event for arguments
			if update.ArgsDelta != "" {
				argsEvent := &ToolCallArgumentsStreamedEvent{
					BaseEvent: BaseEvent{
						Type:      "ToolCallArgumentsStreamed",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID: update.ID,
					Delta:      update.ArgsDelta,
				}
				if err := h.emitDomainEvent(argsEvent, eventHandler); err != nil {
					return err
				}
			}
		}
	}

	// Get accumulated tool calls
	toolCallsList := toolCalls.GetAll()

	// Emit ToolCallCompletedEvent for each tool call
	for _, tc := range toolCallsList {
		completedEvent := &ToolCallCompletedEvent{
			BaseEvent: BaseEvent{
				Type:      "ToolCallCompleted",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			ToolCallID: tc.ID,
		}
		if err := h.emitDomainEvent(completedEvent, eventHandler); err != nil {
			return err
		}
	}

	// Store assistant message and handle tool calls
	if toolCalls.HasToolCalls() {

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
		// Exception: If agent emitted routing events, stop iteration
		// The worker pool will handle activating the appropriate agent
		for _, toolCall := range toolCallsList {
			toolName := toolCall.Function.Name
			if toolName == "emit_math_question" ||
				toolName == "emit_physics_question" ||
				toolName == "emit_chemistry_question" ||
				toolName == "emit_question_answered" {
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
// For collaboration tools, it also emits domain events
func (h *Agent) executeToolCall(threadID string, toolCall domain.ToolCall, eventHandler DomainEventHandler) (string, error) {
	// Handle collaboration tools that need to emit domain events
	if err := h.emitCollaborationEvents(threadID, toolCall, eventHandler); err != nil {
		return "", err
	}

	// Execute the tool using the executor
	return h.toolExecutor.Execute(toolCall)
}

// emitCollaborationEvents emits domain events for collaboration tools
func (h *Agent) emitCollaborationEvents(threadID string, toolCall domain.ToolCall, eventHandler DomainEventHandler) error {
	toolName := toolCall.Function.Name

	// Get whiteboard state for events
	getWhiteboardState := func() string {
		thread, exists := h.state.Get(threadID)
		if exists {
			return thread.WhiteboardContent
		}
		return ""
	}

	switch toolName {
	case "emit_math_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return fmt.Errorf("failed to parse emit_math_question arguments: %w", err)
		}

		return h.emitDomainEvent(
			NewMathQuestionEvent(threadID, input.Question, h.config.Name, input.Context, getWhiteboardState()),
			eventHandler,
		)

	case "emit_physics_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return fmt.Errorf("failed to parse emit_physics_question arguments: %w", err)
		}

		return h.emitDomainEvent(
			NewPhysicsQuestionEvent(threadID, input.Question, h.config.Name, input.Context, getWhiteboardState()),
			eventHandler,
		)

	case "emit_chemistry_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return fmt.Errorf("failed to parse emit_chemistry_question arguments: %w", err)
		}

		return h.emitDomainEvent(
			NewChemistryQuestionEvent(threadID, input.Question, h.config.Name, input.Context, getWhiteboardState()),
			eventHandler,
		)

	case "emit_question_answered":
		var input struct {
			Subject    string `json:"subject"`
			IsComplete bool   `json:"is_complete"`
			Summary    string `json:"summary"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return fmt.Errorf("failed to parse emit_question_answered arguments: %w", err)
		}

		return h.emitDomainEvent(
			NewQuestionAnsweredEvent(threadID, input.Subject, input.Summary, h.config.Name, input.IsComplete, getWhiteboardState()),
			eventHandler,
		)

	case "write_whiteboard":
		var input struct {
			Content string `json:"content"`
			Action  string `json:"action"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return fmt.Errorf("failed to parse write_whiteboard arguments: %w", err)
		}

		return h.emitDomainEvent(
			NewWhiteboardUpdatedEvent(threadID, h.config.Name, input.Content, input.Action),
			eventHandler,
		)

	case "emit_event":
		var input struct {
			EventName   string                 `json:"event_name"`
			Payload     map[string]interface{} `json:"payload"`
			Description string                 `json:"description"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return fmt.Errorf("failed to parse emit_event arguments: %w", err)
		}

		return h.emitDomainEvent(
			NewCustomEventEmitted(threadID, input.EventName, h.config.Name, input.Description, input.Payload),
			eventHandler,
		)
	}

	return nil
}
