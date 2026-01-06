package chatwrite

import (
	"context"
	"fmt"
	"iter"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/llm"
)

// ChatClient defines the interface for LLM providers (OpenAI, Anthropic, Ollama, etc.)
type ChatClient interface {
	GetStreamingResponse(
		ctx context.Context,
		messages []llm.ChatMessage,
		options *llm.ChatOptions,
	) iter.Seq2[llm.ChatResponseUpdate, error]
}

// EventStore is the source of truth in the event sourcing architecture
type EventStore interface {
	Append(ctx context.Context, streamID string, events ...core.Event) (int64, error)
	ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]core.Event, error)
}

// ThreadStore provides read access to thread state
type ThreadStore interface {
	GetAgentThread(threadID string) (core.AgentThread, bool)
	GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error)
	Get(threadID string) (*domain.Thread, bool)
	Messages(threadID string) []domain.Message
	All() []domain.Thread
	Update(event core.Event) error
}

// ToolCallHandler is called after a tool is executed, allowing custom event emission
type ToolCallHandler func(threadID string, agentName string, toolCall domain.ToolCall, result string, eventEmitter func(core.Event) error) error

// ShouldContinueIterationFn determines if the agent should continue iterating after tool calls
type ShouldContinueIterationFn func(toolCalls []domain.ToolCall) bool

// ChatWriteService handles processing of user messages using pure event sourcing
// Each agent is responsible for registering its own event notifications
type ChatWriteService struct {
	config                  core.AgentConfig
	chatClient              ChatClient
	writeModel              EventStore
	readModel               ThreadStore
	toolRegistry            *core.ToolRegistry
	notifier                core.EventNotifier // Agent's own notifier
	toolCallHandler         ToolCallHandler
	shouldContinueIteration ShouldContinueIterationFn
}

// Option is a functional option for configuring ChatWriteService
type Option func(*ChatWriteService)

// WithConfig configures the agent with a complete configuration
func WithConfig(config core.AgentConfig) Option {
	return func(h *ChatWriteService) {
		h.config = config
	}
}

// WithToolCallHandler configures the agent with a tool call handler for custom event emission
func WithToolCallHandler(handler ToolCallHandler) Option {
	return func(h *ChatWriteService) {
		h.toolCallHandler = handler
	}
}

// WithShouldContinueIteration configures the agent with a function to determine if iteration should continue
func WithShouldContinueIteration(fn ShouldContinueIterationFn) Option {
	return func(h *ChatWriteService) {
		h.shouldContinueIteration = fn
	}
}

// New creates a new chat write service with event sourcing (no UI dependencies)
func New(chatClient ChatClient, writeModel EventStore, readModel ThreadStore, opts ...Option) *ChatWriteService {
	// Create tool registry
	toolRegistry := core.NewToolRegistry()

	h := &ChatWriteService{
		chatClient:   chatClient,
		writeModel:   writeModel,
		readModel:    readModel,
		config:       core.DefaultConfig("agent"), // Default config
		toolRegistry: toolRegistry,                // Tool registry
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
func (h *ChatWriteService) GetNewThread() core.AgentThread {
	return core.NewInMemoryThread()
}

// DeserializeThread deserializes an agent thread from its JSON serialized representation.
// This method enables restoration of conversation threads from previously saved state,
// allowing conversations to resume across application restarts or be migrated between
// different agent instances.
func (h *ChatWriteService) DeserializeThread(data []byte) (core.AgentThread, error) {
	thread, err := core.NewInMemoryThreadFromState(data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize thread: %w", err)
	}
	return thread, nil
}

// DomainEventHandler is called for each domain event emitted by the agent
// This allows external systems to react to events (e.g., UI projections, logging, etc.)
type DomainEventHandler func(event core.Event) error

// RunWithThread processes the LLM streaming response using an AgentThread for message management.
// This is the modern API that provides better separation of concerns and supports advanced features
// like message reduction, custom storage, and serialization.
//
// The thread parameter can be obtained from GetNewThread() or DeserializeThread().
func (h *ChatWriteService) RunWithThread(
	ctx context.Context,
	threadID string,
	messageID string,
	thread core.AgentThread,
	eventHandler DomainEventHandler,
	runOpts core.RunOptions,
) error {
	// Get messages from thread
	messages := thread.GetMessages()
	return h.runIteration(ctx, threadID, messageID, messages, thread, eventHandler, runOpts)
}

// runIteration processes a single LLM iteration and recursively continues if tools are called
// thread parameter is optional - if provided, it will be used for message management
func (h *ChatWriteService) runIteration(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []domain.Message,
	thread core.AgentThread,
	eventHandler DomainEventHandler,
	runOpts core.RunOptions,
) error {
	// Check if client disconnected before starting LLM call
	select {
	case <-ctx.Done():
		return fmt.Errorf("client disconnected: %w", ctx.Err())
	default:
	}

	// Emit AssistantMessageStartedEvent
	startEvent := &core.AssistantMessageStartedEvent{
		BaseEvent: core.BaseEvent{
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

	// Build chat options from agent config and run options, applying middleware
	chatOptions, reqCtx, err := core.ToChatOptionsWithMiddleware(ctx, h.config, chatMessages, runOpts)
	if err != nil {
		return fmt.Errorf("failed to apply middleware: %w", err)
	}

	// Check if middleware terminated early
	if reqCtx.Terminate {
		return nil
	}

	// Create a dynamic tool registry that includes middleware tools
	dynamicRegistry := core.NewToolRegistry()
	for _, tool := range reqCtx.GetEffectiveTools() {
		dynamicRegistry.Register(tool)
	}

	// Create a tool executor with the dynamic registry
	dynamicExecutor := core.NewRegistryToolExecutor(ctx, dynamicRegistry)

	// Get streaming response from ChatClient
	streamIter := h.chatClient.GetStreamingResponse(ctx, chatMessages, chatOptions)

	// Track complete content for storage
	var completeContent string

	// Use accumulator to track streaming tool calls
	toolCalls := core.NewToolCallAccumulator()

	// Process stream updates using iterator
	for update, err := range streamIter {
		// Check if client disconnected during streaming
		select {
		case <-ctx.Done():
			// Save partial content as event before returning
			if completeContent != "" {
				evt := &core.AssistantMessageCompletedEvent{
					BaseEvent: core.BaseEvent{
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

			contentEvent := &core.AssistantContentStreamedEvent{
				BaseEvent: core.BaseEvent{
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
				toolStartEvent := &core.ToolCallStartedEvent{
					BaseEvent: core.BaseEvent{
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
				argsEvent := &core.ToolCallArgumentsStreamedEvent{
					BaseEvent: core.BaseEvent{
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
		completedEvent := &core.ToolCallCompletedEvent{
			BaseEvent: core.BaseEvent{
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
		assistantEvent := &core.AssistantMessageCompletedEvent{
			BaseEvent: core.BaseEvent{
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
			// Execute the tool call using dynamic executor with middleware tools
			result, err := h.executeToolCallWithExecutor(threadID, toolCall, eventHandler, dynamicExecutor)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}

			// Emit ToolCallExecutedEvent
			toolResultID := core.GenerateID()
			toolEvent := &core.ToolCallExecutedEvent{
				BaseEvent: core.BaseEvent{
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

		// Get updated thread state with tool results
		stateWithAccess, ok := h.readModel.(core.ThreadStateWithAgentAccess)
		if !ok {
			return fmt.Errorf("state does not implement ThreadStateWithAgentAccess")
		}

		updatedThread, exists := stateWithAccess.GetAgentThread(threadID)
		if !exists {
			return fmt.Errorf("thread %s not found after tool execution", threadID)
		}

		// Generate new message ID for the continuation
		continuationMessageID := core.GenerateID()

		// Recursively call to process the LLM response with tool results
		// Pass the same runOpts to maintain consistency across iterations
		return h.runIteration(ctx, threadID, continuationMessageID, updatedThread.GetMessages(), thread, eventHandler, runOpts)
	}

	// No tool calls, just emit the completed message event
	assistantEvent := &core.AssistantMessageCompletedEvent{
		BaseEvent: core.BaseEvent{
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
func (h *ChatWriteService) emitDomainEvent(event core.Event, eventHandler DomainEventHandler) error {
	// Append to event store (this will automatically update projections via ProjectionEventStore)
	_, err := h.writeModel.Append(context.Background(), event.AggregateID(), event)
	if err != nil {
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

// executeToolCallWithExecutor executes a tool call using a dynamic executor that includes middleware tools
func (h *ChatWriteService) executeToolCallWithExecutor(threadID string, toolCall domain.ToolCall, eventHandler DomainEventHandler, executor core.ToolExecutor) (string, error) {
	// Execute the tool using the provided executor
	result, err := executor.Execute(toolCall)
	if err != nil {
		return result, err
	}

	// If a tool call handler is configured, call it for custom event emission
	if h.toolCallHandler != nil {
		emitFn := func(event core.Event) error {
			return h.emitDomainEvent(event, eventHandler)
		}
		if err := h.toolCallHandler(threadID, h.config.Name, toolCall, result, emitFn); err != nil {
			return result, err
		}
	}

	return result, nil
}

// CreateThread creates a new chat thread
func (h *ChatWriteService) CreateThread(ctx context.Context, threadID, title string) error {
	// Check if thread already exists
	if _, exists := h.readModel.Get(threadID); exists {
		return nil // Thread already exists, no-op
	}

	event := &core.ThreadCreatedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "ThreadCreated",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		Title: title,
	}

	_, err := h.writeModel.Append(ctx, threadID, event)
	return err
}

// AddUserMessage adds a user message to a thread
func (h *ChatWriteService) AddUserMessage(ctx context.Context, threadID, messageID, content string) error {
	event := &core.UserMessageAddedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "UserMessageAdded",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
		Content:   content,
	}

	_, err := h.writeModel.Append(ctx, threadID, event)
	return err
}

// UpdateThreadTitle updates the title of a thread
func (h *ChatWriteService) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) error {
	event := &core.ThreadTitleUpdatedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "ThreadTitleUpdated",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		NewTitle: newTitle,
	}

	_, err := h.writeModel.Append(ctx, threadID, event)
	return err
}

// DeleteThread deletes a thread
func (h *ChatWriteService) DeleteThread(ctx context.Context, threadID string) error {
	// Check if thread exists
	if _, exists := h.readModel.Get(threadID); !exists {
		return domain.ErrThreadNotFound
	}

	event := &core.ThreadDeletedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "ThreadDeleted",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
	}

	_, err := h.writeModel.Append(ctx, threadID, event)
	return err
}
