package sdk

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/llm"
)

// ToolCallHandler is called after a tool is executed, allowing custom event emission
type ToolCallHandler func(threadID string, agentName string, toolCall domain.ToolCall, result string, eventEmitter func(Event) error) error

// ShouldContinueIterationFn determines if the agent should continue iterating after tool calls
type ShouldContinueIterationFn func(toolCalls []domain.ToolCall) bool

// Agent handles processing of user messages using pure event sourcing
// Each agent is responsible for registering its own event notifications
type Agent struct {
	config                  AgentConfig
	chatClient              ChatClient
	log                     EventLog
	state                   ThreadState
	toolRegistry            *ToolRegistry
	toolExecutor            ToolExecutor
	notifier                EventNotifier // Agent's own notifier
	toolCallHandler         ToolCallHandler
	shouldContinueIteration ShouldContinueIterationFn
}

// Option is a functional option for configuring Agent
type Option func(*Agent)

// WithConfig configures the agent with a complete configuration
func WithConfig(config AgentConfig) Option {
	return func(h *Agent) {
		h.config = config
	}
}

// WithToolCallHandler configures the agent with a tool call handler for custom event emission
func WithToolCallHandler(handler ToolCallHandler) Option {
	return func(h *Agent) {
		h.toolCallHandler = handler
	}
}

// WithShouldContinueIteration configures the agent with a function to determine if iteration should continue
func WithShouldContinueIteration(fn ShouldContinueIterationFn) Option {
	return func(h *Agent) {
		h.shouldContinueIteration = fn
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

// GetNewThread creates a new conversation thread that is compatible with this agent.
// This method creates a fresh conversation thread that can be used to maintain state
// and context for interactions with this agent. Each thread represents an independent
// conversation session.
func (h *Agent) GetNewThread() AgentThread {
	return NewInMemoryThread()
}

// DeserializeThread deserializes an agent thread from its JSON serialized representation.
// This method enables restoration of conversation threads from previously saved state,
// allowing conversations to resume across application restarts or be migrated between
// different agent instances.
func (h *Agent) DeserializeThread(data []byte) (AgentThread, error) {
	thread, err := NewInMemoryThreadFromState(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize thread: %w", err)
	}
	return thread, nil
}

// DomainEventHandler is called for each domain event emitted by the agent
// This allows external systems to react to events (e.g., UI projections, logging, etc.)
type DomainEventHandler func(event Event) error

// RunWithThread processes the LLM streaming response using an AgentThread for message management.
// This is the modern API that provides better separation of concerns and supports advanced features
// like message reduction, custom storage, and serialization.
//
// The thread parameter can be obtained from GetNewThread() or DeserializeThread().
func (h *Agent) RunWithThread(
	ctx context.Context,
	threadID string,
	messageID string,
	thread AgentThread,
	eventHandler DomainEventHandler,
	runOpts RunOptions,
) error {
	// Get messages from thread
	messages := thread.GetMessages()
	return h.runIteration(ctx, threadID, messageID, messages, thread, eventHandler, runOpts)
}

// runIteration processes a single LLM iteration and recursively continues if tools are called
// thread parameter is optional - if provided, it will be used for message management
func (h *Agent) runIteration(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []domain.Message,
	thread AgentThread,
	eventHandler DomainEventHandler,
	runOpts RunOptions,
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

	// Build chat options from agent config and run options
	chatOptions := ToChatOptions(h.config, runOpts)

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
		// Check if we should continue iteration (allows external control)
		if h.shouldContinueIteration != nil && !h.shouldContinueIteration(toolCallsList) {
			// External logic determined we should stop
			return nil
		}

		// Get updated messages including tool results from state
		updatedMessages := h.state.Messages(threadID)

		// Generate new message ID for the continuation
		continuationMessageID := GenerateID()

		// Recursively call to process the LLM response with tool results
		// Pass the same runOpts to maintain consistency across iterations
		return h.runIteration(ctx, threadID, continuationMessageID, updatedMessages, thread, eventHandler, runOpts)
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
func (h *Agent) executeToolCall(threadID string, toolCall domain.ToolCall, eventHandler DomainEventHandler) (string, error) {
	// Execute the tool using the executor
	result, err := h.toolExecutor.Execute(toolCall)
	if err != nil {
		return result, err
	}

	// If a tool call handler is configured, call it for custom event emission
	if h.toolCallHandler != nil {
		emitFn := func(event Event) error {
			return h.emitDomainEvent(event, eventHandler)
		}
		if err := h.toolCallHandler(threadID, h.config.Name, toolCall, result, emitFn); err != nil {
			return result, err
		}
	}

	return result, nil
}
