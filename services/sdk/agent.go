package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/llm"
)

// isCollaborationTool checks if a tool is a collaboration/routing tool.
// This uses pattern matching to avoid import cycles with eventbus.
func isCollaborationTool(toolName string) bool {
	// Emission tools follow pattern: emit_<subject>_question
	if strings.HasPrefix(toolName, "emit_") && strings.HasSuffix(toolName, "_question") {
		return true
	}
	// Special collaboration tools
	return toolName == "emit_question_answered" ||
		toolName == "write_whiteboard" ||
		toolName == "read_whiteboard"
}

// isEmissionTool checks if a tool is an emit_*_question tool
func isEmissionTool(toolName string) bool {
	return strings.HasPrefix(toolName, "emit_") && strings.HasSuffix(toolName, "_question")
}

// eventTypeFromToolName extracts the event type from an emission tool name.
// e.g., "emit_math_question" -> "question.math"
func eventTypeFromToolName(toolName string) agents.EventType {
	if len(toolName) < 14 { // len("emit_") + len("_question")
		return ""
	}
	slug := toolName[5 : len(toolName)-9] // strip "emit_" prefix and "_question" suffix
	return agents.EventType("question." + slug)
}

// createEventFromEmissionTool creates a domain event from an emission tool call.
// This handles dynamic emit_*_question tools using pattern matching.
func createEventFromEmissionTool(threadID, agentName string, toolCall domain.ToolCall, getWhiteboardState func(string) string) (*agents.DomainEvent, error) {
	toolName := toolCall.Function.Name

	// Parse the arguments
	var params struct {
		Question string `json:"question"`
		Request  string `json:"request"` // Alternative field name
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &params); err != nil {
		return nil, fmt.Errorf("failed to parse %s arguments: %w", toolName, err)
	}

	// Use "question" or "request" field
	questionValue := params.Question
	if questionValue == "" {
		questionValue = params.Request
	}

	// Derive event type from tool name
	eventType := eventTypeFromToolName(toolName)

	return &agents.DomainEvent{
		ID:        agents.GenerateEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"question":         questionValue,
			"emitted_by":       agentName,
			"context":          params.Context,
			"whiteboard_state": getWhiteboardState(threadID),
		},
	}, nil
}

// createHandoffEventFromTool creates a handoff event for an emission tool.
func createHandoffEventFromTool(threadID, fromAgent, toolName string) *agents.DomainEvent {
	if !isEmissionTool(toolName) {
		return nil
	}

	// Extract subject from tool name: "emit_math_question" -> "math"
	slug := toolName[5 : len(toolName)-9]
	targetAgent := slug + "-instructor"
	reason := strings.Title(slug) + " question requires specialist"

	// Special case for linearalgebra -> professor
	if slug == "linearalgebra" {
		targetAgent = "linearalgebra-professor"
	}

	return agents.NewAgentHandoffEvent(threadID, fromAgent, targetAgent, reason)
}

// Agent handles processing of user messages using pure event sourcing
// Each agent is responsible for registering its own event notifications
type Agent struct {
	config       AgentConfig
	chatClient   ChatClient
	log          EventLog
	state        ThreadState
	toolRegistry *ToolRegistry
	toolExecutor ToolExecutor
	notifier     EventNotifier // Agent's own notifier
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
func NewAgent(chatClient ChatClient, log EventLog, state ThreadState, opts ...Option) *Agent {
	// Create tool registry
	toolRegistry := NewToolRegistry()

	h := &Agent{
		chatClient:   chatClient,
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

	// Set notifier from config
	h.notifier = h.config.Notifier

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

	// Convert domain messages to ChatMessage format
	chatMessages := llm.FromDomainMessages(messages)

	// Build chat options from agent config
	chatOptions := ToChatOptions(h.config)

	// Get streaming response from ChatClient
	streamIter := h.chatClient.GetStreamingResponse(ctx, chatMessages, chatOptions)

	// Track complete content for storage
	var completeContent string

	// Use accumulator to track streaming tool calls
	toolCalls := NewToolCallAccumulator()

	// Process stream updates using iterator
	for update, err := range streamIter {
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
		}

		// Check for errors
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		// Handle text content - emit streaming event for each delta
		textDelta := update.Text()
		if textDelta != "" {
			completeContent += textDelta

			contentEvent := &AssistantContentStreamedEvent{
				BaseEvent: BaseEvent{
					Type:      "AssistantContentStreamed",
					Timestamp: time.Now(),
					ThreadID:  threadID,
				},
				MessageID: messageID,
				Delta:     textDelta,
			}
			if err := h.emitDomainEvent(contentEvent, eventHandler); err != nil {
				return err
			}
		}

		// Handle tool calls using accumulator
		tcUpdate, hasTool := toolCalls.ProcessUpdate(update)
		if hasTool {
			if tcUpdate.IsNew {
				// Emit ToolCallStartedEvent for new tool calls
				toolStartEvent := &ToolCallStartedEvent{
					BaseEvent: BaseEvent{
						Type:      "ToolCallStarted",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID:      tcUpdate.ID,
					ToolCallName:    tcUpdate.Name,
					ParentMessageID: messageID,
					AgentName:       h.config.Name,
				}
				if err := h.emitDomainEvent(toolStartEvent, eventHandler); err != nil {
					return err
				}
			}

			// Emit streaming event for arguments
			if tcUpdate.ArgsDelta != "" {
				argsEvent := &ToolCallArgumentsStreamedEvent{
					BaseEvent: BaseEvent{
						Type:      "ToolCallArgumentsStreamed",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID: tcUpdate.ID,
					Delta:      tcUpdate.ArgsDelta,
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
			if isCollaborationTool(toolCall.Function.Name) {
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

	// Notify via agent's own notifier first (if configured)
	if h.notifier != nil {
		if err := h.notifier.HandleDomainEvent(event); err != nil {
			return err
		}
	}

	// Notify external handler (e.g., for routing decisions)
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
	toolName := toolCall.Function.Name

	// Create whiteboard state getter
	getWhiteboardState := func(tid string) string {
		thread, exists := h.state.Get(tid)
		if exists {
			return thread.WhiteboardContent
		}
		return ""
	}

	// Handle emit_*_question tools (dynamically routed via EventBus)
	if isEmissionTool(toolName) {
		event, err := createEventFromEmissionTool(threadID, h.config.Name, toolCall, getWhiteboardState)
		if err != nil {
			return "", err
		}
		if err := h.emitDomainEvent(event, eventHandler); err != nil {
			return "", err
		}

		// Emit agent handoff event
		if handoffEvent := createHandoffEventFromTool(threadID, h.config.Name, toolName); handoffEvent != nil {
			if err := h.emitDomainEvent(handoffEvent, eventHandler); err != nil {
				return "", err
			}
		}

		return "{\"status\": \"event_emitted\"}", nil
	}

	// Handle special collaboration tools (emit_question_answered, write_whiteboard)
	if toolName == "emit_question_answered" || toolName == "write_whiteboard" {
		event, err := tools.HandleSpecialToolCall(threadID, h.config.Name, toolCall, getWhiteboardState)
		if err != nil {
			return "", err
		}
		if err := h.emitDomainEvent(event, eventHandler); err != nil {
			return "", err
		}

		// Emit agent handoff event for emit_question_answered
		if handoffEvent := tools.GetSpecialToolHandoffEvent(threadID, h.config.Name, toolName); handoffEvent != nil {
			if err := h.emitDomainEvent(handoffEvent, eventHandler); err != nil {
				return "", err
			}
		}

		return "{\"status\": \"event_emitted\"}", nil
	}

	// Handle custom event emission
	if toolCall.Function.Name == "emit_event" {
		var input struct {
			EventName   string                 `json:"event_name"`
			Payload     map[string]interface{} `json:"payload"`
			Description string                 `json:"description"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return "", fmt.Errorf("failed to parse emit_event arguments: %w", err)
		}
		event := NewCustomEventEmitted(threadID, input.EventName, h.config.Name, input.Description, input.Payload)
		if err := h.emitDomainEvent(event, eventHandler); err != nil {
			return "", err
		}
		return "{\"status\": \"event_emitted\"}", nil
	}

	// Execute the tool using the executor
	return h.toolExecutor.Execute(toolCall)
}
