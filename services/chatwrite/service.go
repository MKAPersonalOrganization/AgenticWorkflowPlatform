package chatwrite

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/llm"
	"school-with-ai/services/chatwrite/rag"
)

var (
	// ErrThreadDeserializationFailed is returned when thread deserialization fails.
	ErrThreadDeserializationFailed = errors.New("failed to deserialize thread")
	// ErrStateNotThreadStateWithAgentAccess is returned when state doesn't implement ThreadStateWithAgentAccess.
	ErrStateNotThreadStateWithAgentAccess = errors.New("state does not implement ThreadStateWithAgentAccess")
)

// ChatClient defines the interface for LLM providers (OpenAI, Anthropic, Ollama, etc.)
type ChatClient interface {
	GetStreamingResponse(
		ctx context.Context,
		messages []llm.ChatMessage,
		options *llm.ChatOptions,
	) iter.Seq2[llm.ChatResponseUpdate, error]
}

// ThreadStore provides read access to thread state.
type ThreadStore interface {
	GetAgentThread(threadID string) (core.AgentThread, bool)
	GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error)
	Get(threadID string) (*domain.Thread, bool)
	Messages(threadID string) []domain.Message
	All() []domain.Thread
	Update(event core.Event) error
}

// ShouldContinueIterationFn determines if the agent should continue iterating after tool calls.
type ShouldContinueIterationFn func(toolCalls []domain.ToolCall) bool

// ChatWriteService handles processing of user messages using pure event sourcing
// Each agent is responsible for registering its own event notifications.
type ChatWriteService struct {
	log                     *logger.Logger
	config                  core.AgentConfig
	chatClient              ChatClient
	writeModel              core.EventStore
	readModel               ThreadStore
	threadRepo              *domain.ThreadRepository // Repository for thread aggregate persistence
	toolRegistry            *core.ToolRegistry
	notifier                core.EventNotifier // Agent's own notifier
	shouldContinueIteration ShouldContinueIterationFn
	reqCtxCapture           **core.RequestContext // Captures the request context for handoff detection
	ragService              rag.RAG               // RAG service for retrieving relevant context
}

// Option is a functional option for configuring ChatWriteService.
type Option func(*ChatWriteService)

// WithConfig configures the agent with a complete configuration.
func WithConfig(config core.AgentConfig) Option {
	return func(h *ChatWriteService) {
		h.config = config
	}
}

// WithShouldContinueIteration configures the agent with a function to determine if iteration should continue.
func WithShouldContinueIteration(fn ShouldContinueIterationFn) Option {
	return func(h *ChatWriteService) {
		h.shouldContinueIteration = fn
	}
}

// WithRequestContextCapture configures the agent to capture the request context for handoff detection.
func WithRequestContextCapture(capture **core.RequestContext) Option {
	return func(h *ChatWriteService) {
		h.reqCtxCapture = capture
	}
}

// WithRAG configures the agent with a RAG service for context retrieval.
func WithRAG(ragService rag.RAG) Option {
	return func(h *ChatWriteService) {
		h.ragService = ragService
	}
}

// New creates a new chat write service with event sourcing (no UI dependencies).
func New(log *logger.Logger, chatClient ChatClient, writeModel core.EventStore, readModel ThreadStore, opts ...Option) *ChatWriteService {
	// Create tool registry
	toolRegistry := core.NewToolRegistry()

	h := &ChatWriteService{
		log:          log,
		chatClient:   chatClient,
		writeModel:   writeModel,
		readModel:    readModel,
		threadRepo:   domain.NewThreadRepository(writeModel), // Create repository once
		config:       core.DefaultConfig("agent"),            // Default config
		toolRegistry: toolRegistry,                           // Tool registry
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
		return nil, fmt.Errorf("%w: %w", ErrThreadDeserializationFailed, err)
	}

	return thread, nil
}

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
	runOpts core.RunOptions,
) error {
	// Get messages from thread
	messages := thread.GetMessages()

	return h.runIteration(ctx, threadID, messageID, messages, thread, runOpts)
}

// runIteration processes a single LLM iteration and recursively continues if tools are called
// thread parameter is optional - if provided, it will be used for message management.
func (h *ChatWriteService) runIteration(
	ctx context.Context,
	threadID string,
	messageID string,
	messages []domain.Message,
	thread core.AgentThread,
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
			Type:      "AssistantMessageStartedEvent",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
	}
	if err := h.emitUIEvent(startEvent); err != nil {
		return err
	}

	// Convert domain messages to ChatMessage format
	llmMsgs := convertCoreMsgsToLLMMsgs(messages)
	chatMessages := llm.FromDomainMessages(llmMsgs)

	// Build chat options from agent config and run options, applying middleware
	chatOptions, reqCtx, err := core.ToChatOptionsWithMiddleware(ctx, h.config, chatMessages, threadID, runOpts)
	if err != nil {
		return fmt.Errorf("failed to apply middleware: %w", err)
	}

	// Capture request context if configured (for handoff detection)
	if h.reqCtxCapture != nil {
		*h.reqCtxCapture = reqCtx
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
						Type:      "AssistantMessageCompletedEvent",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					MessageID: messageID,
					Content:   completeContent,
					AgentName: h.config.Name,
				}
				_ = h.emitUIEvent(evt)
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
					Type:      "AssistantContentStreamedEvent",
					Timestamp: time.Now(),
					ThreadID:  threadID,
				},
				MessageID: messageID,
				Delta:     textDelta,
			}
			err := h.emitUIEvent(contentEvent)
			if err != nil {
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
						Type:      "ToolCallStartedEvent",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID:      tcUpdate.ID,
					ToolCallName:    tcUpdate.Name,
					ParentMessageID: messageID,
					AgentName:       h.config.Name,
				}
				err := h.emitUIEvent(toolStartEvent)
				if err != nil {
					return err
				}
			}

			// Emit streaming event for arguments
			if tcUpdate.ArgsDelta != "" {
				argsEvent := &core.ToolCallArgumentsStreamedEvent{
					BaseEvent: core.BaseEvent{
						Type:      "ToolCallArgumentsStreamedEvent",
						Timestamp: time.Now(),
						ThreadID:  threadID,
					},
					ToolCallID: tcUpdate.ID,
					Delta:      tcUpdate.ArgsDelta,
				}
				err := h.emitUIEvent(argsEvent)
				if err != nil {
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
				Type:      "ToolCallCompletedEvent",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			ToolCallID: tc.ID,
		}
		err := h.emitUIEvent(completedEvent)
		if err != nil {
			return err
		}
	}

	// Store assistant message and handle tool calls
	if toolCalls.HasToolCalls() {
		// Persist assistant message with tool calls as domain event
		err = h.addAssistantMessage(ctx, threadID, messageID, completeContent, toolCallsList)
		if err != nil {
			return fmt.Errorf("failed to add assistant message: %w", err)
		}

		// Emit UI event for real-time notification
		assistantEvent := &core.AssistantMessageCompletedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "AssistantMessageCompletedEvent",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			MessageID: messageID,
			Content:   completeContent,
			ToolCalls: toolCallsList,
			AgentName: h.config.Name,
		}
		err := h.emitUIEvent(assistantEvent)
		if err != nil {
			return err
		}

		// For each tool call: execute and emit result
		for _, toolCall := range toolCallsList {
			// Execute the tool call using dynamic executor with middleware tools
			result, err := h.executeToolCallWithExecutor(threadID, toolCall, dynamicExecutor)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}

			// Persist tool result as domain event
			toolResultID := core.GenerateID()
			err = h.addToolResultMessage(ctx, threadID, toolResultID, toolCall.ID, result)
			if err != nil {
				return fmt.Errorf("failed to add tool result message: %w", err)
			}

			// Emit UI event for real-time notification
			toolEvent := &core.ToolCallExecutedEvent{
				BaseEvent: core.BaseEvent{
					Type:      "ToolCallExecutedEvent",
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
			if err := h.emitUIEvent(toolEvent); err != nil {
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
			return ErrStateNotThreadStateWithAgentAccess
		}

		updatedThread, exists := stateWithAccess.GetAgentThread(threadID)
		if !exists {
			// For query-time projections, try GetOrCreateAgentThread instead
			var err error
			updatedThread, err = stateWithAccess.GetOrCreateAgentThread(ctx, threadID)
			if err != nil {
				return fmt.Errorf("failed to get/create thread %s after tool execution: %w", threadID, err)
			}
		}

		// Generate new message ID for the continuation
		continuationMessageID := core.GenerateID()

		// Recursively call to process the LLM response with tool results
		// Pass the same runOpts to maintain consistency across iterations
		return h.runIteration(ctx, threadID, continuationMessageID, updatedThread.GetMessages(), thread, runOpts)
	}

	// No tool calls - this is the final answer, persist it
	err = h.addAssistantMessage(ctx, threadID, messageID, completeContent, nil)
	if err != nil {
		return fmt.Errorf("failed to add final assistant message: %w", err)
	}

	// Emit UI event for real-time notification
	assistantEvent := &core.AssistantMessageCompletedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "AssistantMessageCompletedEvent",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
		Content:   completeContent,
		AgentName: h.config.Name,
	}
	if err := h.emitUIEvent(assistantEvent); err != nil {
		return err
	}

	return nil
}

// emitUIEvent sends real-time UI events to the notifier without persisting them.
// UI events are ephemeral notifications for real-time updates, not domain state.
func (h *ChatWriteService) emitUIEvent(event core.Event) error {
	// Only notify via agent's notifier (if configured)
	// Do NOT save UI events to the event store!
	if h.notifier != nil {
		err := h.notifier.HandleDomainEvent(event)
		if err != nil {
			return fmt.Errorf("failed to notify UI: %w", err)
		}
	}

	return nil
}

// executeToolCallWithExecutor executes a tool call using a dynamic executor that includes middleware tools.
func (h *ChatWriteService) executeToolCallWithExecutor(threadID string, toolCall domain.ToolCall, executor core.ToolExecutor) (string, error) {
	// Execute the tool using the provided executor
	result, err := executor.Execute(toolCall)
	if err != nil {
		return result, fmt.Errorf("failed to execute tool call %s: %w", toolCall.Function.Name, err)
	}

	return result, nil
}

// CreateThread creates a new chat thread using event sourcing.
func (h *ChatWriteService) CreateThread(ctx context.Context, threadID, title string) error {
	h.log.Debug(ctx, "CreateThread called",
		"threadID", threadID,
		"title", title)

	// Try to load existing thread first (idempotency check)
	existingThread, err := h.threadRepo.Load(ctx, threadID)
	if err == nil {
		h.log.Debug(ctx, "Thread already exists, skipping create",
			"threadID", threadID,
			"version", existingThread.GetVersion())
		// Thread already exists, this is idempotent
		return nil
	}
	if err != nil && !errors.Is(err, domain.ErrThreadNotFound) {
		return fmt.Errorf("failed to check existing thread: %w", err)
	}
	h.log.Debug(ctx, "Thread does not exist, creating new thread", "threadID", threadID)

	// Create thread using static factory method (like ShoppingCart.Open)
	thread, err := domain.OpenThread(threadID, "system", title) // TODO: Get userID from context
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}

	h.log.Debug(ctx, "Thread created, saving with uncommitted events",
		"threadID", threadID,
		"eventCount", len(thread.DequeueUncommittedEvents()))

	// Save the new thread
	err = h.threadRepo.SaveNew(ctx, thread)
	if err != nil {
		h.log.Debug(ctx, "SaveNew failed",
			"threadID", threadID,
			"error", err)
		// If concurrency conflict, thread was created by another request - this is fine
		if errors.Is(err, core.ErrConcurrencyConflict) {
			h.log.Debug(ctx, "Concurrency conflict, thread already created", "threadID", threadID)
			return nil
		}
		return fmt.Errorf("failed to save new thread: %w", err)
	}
	h.log.Debug(ctx, "Thread created successfully", "threadID", threadID)

	// Emit UI event for frontend notification
	_ = h.emitUIEvent(&core.ThreadCreatedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "ThreadCreatedEvent",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		Title: title,
	})

	return nil
}

// AddUserMessage adds a user message to a thread using event sourcing.
func (h *ChatWriteService) AddUserMessage(ctx context.Context, threadID, messageID, content string) error {
	h.log.Debug(ctx, "AddUserMessage called",
		"threadID", threadID,
		"messageID", messageID)

	// Load existing aggregate
	thread, err := h.threadRepo.Load(ctx, threadID)

	// Handle thread not found - create it
	if errors.Is(err, domain.ErrThreadNotFound) {
		h.log.Debug(ctx, "Thread not found, creating new thread", "threadID", threadID)

		// Create thread using static factory
		thread, err = domain.OpenThread(threadID, "system", "New Chat") // TODO: Get userID from context
		if err != nil {
			return fmt.Errorf("failed to create thread: %w", err)
		}
	} else if err != nil {
		h.log.Debug(ctx, "Failed to load thread for message",
			"threadID", threadID,
			"error", err)
		return fmt.Errorf("failed to load thread: %w", err)
	} else {
		h.log.Debug(ctx, "Thread loaded for message",
			"threadID", threadID,
			"version", thread.Version,
			"title", thread.Title)
	}

	// Remember if this is a new thread (for choosing Save vs SaveNew)
	isNewThread := thread.GetVersion() == 0

	// Add the user message using aggregate method
	err = thread.AddMessage(messageID, "user", content, nil, "", "", nil)
	if err != nil {
		h.log.Debug(ctx, "Failed to add message",
			"threadID", threadID,
			"messageID", messageID,
			"error", err)
		return fmt.Errorf("failed to add message: %w", err)
	}

	h.log.Debug(ctx, "Message added, saving thread",
		"threadID", threadID,
		"isNewThread", isNewThread)

	// Save the thread
	// For new threads: use SaveNew (expects version 0 in event store)
	// For existing threads: use Save (expects current version)
	if isNewThread {
		h.log.Debug(ctx, "New thread - using SaveNew", "threadID", threadID)
		err = h.threadRepo.SaveNew(ctx, thread)
		if err != nil {
			h.log.Debug(ctx, "SaveNew failed",
				"threadID", threadID,
				"error", err)
			// If concurrency conflict, thread was created by another request
			if errors.Is(err, core.ErrConcurrencyConflict) {
				h.log.Debug(ctx, "Concurrency conflict - thread created by another request", "threadID", threadID)
				return nil // Idempotent - thread exists, no error
			}
			return fmt.Errorf("failed to save new thread: %w", err)
		}
	} else {
		h.log.Debug(ctx, "Existing thread - using Save with version check", "threadID", threadID)
		err = h.threadRepo.Save(ctx, thread)
		if err != nil {
			h.log.Debug(ctx, "Failed to save thread",
				"threadID", threadID,
				"error", err)
			return fmt.Errorf("failed to save thread: %w", err)
		}
	}

	h.log.Debug(ctx, "Thread saved successfully", "threadID", threadID)

	// Emit UI event for frontend notification
	_ = h.emitUIEvent(&core.UserMessageAddedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "UserMessageAddedEvent",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		MessageID: messageID,
		Content:   content,
	})

	return nil
}

// addAssistantMessage adds an assistant message to the thread using event sourcing.
func (h *ChatWriteService) addAssistantMessage(ctx context.Context, threadID, messageID, content string, toolCalls []domain.ToolCall) error {
	thread, err := h.threadRepo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	// Add message using aggregate method
	err = thread.AddMessage(messageID, "assistant", content, toolCalls, "", h.config.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to add assistant message: %w", err)
	}

	return h.threadRepo.Save(ctx, thread)
}

// addToolResultMessage adds a tool result message to the thread using event sourcing.
func (h *ChatWriteService) addToolResultMessage(ctx context.Context, threadID, messageID, toolCallID, result string) error {
	thread, err := h.threadRepo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	// Add message using aggregate method
	err = thread.AddMessage(messageID, "tool", result, nil, toolCallID, h.config.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to add tool result: %w", err)
	}

	return h.threadRepo.Save(ctx, thread)
}

// UpdateThreadTitle updates the title of a thread using event sourcing.
func (h *ChatWriteService) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) error {
	h.log.Debug(ctx, "UpdateThreadTitle called",
		"threadID", threadID,
		"newTitle", newTitle)

	// Load existing aggregate
	thread, err := h.threadRepo.Load(ctx, threadID)
	if err != nil {
		h.log.Debug(ctx, "Failed to load thread for title update",
			"threadID", threadID,
			"error", err)
		return fmt.Errorf("failed to load thread: %w", err)
	}
	h.log.Debug(ctx, "Thread loaded",
		"threadID", threadID,
		"version", thread.GetVersion(),
		"currentTitle", thread.Title)

	// Change title using aggregate method
	err = thread.ChangeTitle(newTitle)
	if err != nil {
		h.log.Debug(ctx, "Failed to change title",
			"threadID", threadID,
			"error", err)
		return fmt.Errorf("failed to change title: %w", err)
	}

	err = h.threadRepo.Save(ctx, thread)
	if err != nil {
		h.log.Debug(ctx, "Failed to save thread after title change",
			"threadID", threadID,
			"error", err)
		return fmt.Errorf("failed to save thread: %w", err)
	}
	h.log.Debug(ctx, "Thread title updated successfully",
		"threadID", threadID,
		"newTitle", newTitle)

	// Emit UI event for frontend notification
	_ = h.emitUIEvent(&core.ThreadTitleUpdatedEvent{
		BaseEvent: core.BaseEvent{
			Type:      "ThreadTitleUpdatedEvent",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		NewTitle: newTitle,
	})

	return nil
}

// DeleteThread deletes a thread using event sourcing (appends ThreadDeleted event).
func (h *ChatWriteService) DeleteThread(ctx context.Context, threadID string) error {
	// Use repository to handle deletion
	err := h.threadRepo.Delete(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}

	return nil
}

// convertCoreMsgToLLMMsg converts core.Message to llm.Message.
// This avoids circular imports between core and llm packages.
func convertCoreMsgToLLMMsg(cm core.Message) llm.Message {
	msg := llm.Message{
		ID:         cm.ID,
		Role:       cm.Role,
		Content:    cm.Content,
		Timestamp:  cm.Timestamp,
		ToolCallID: cm.ToolCallID,
		AgentName:  cm.AgentName,
		Metadata:   cm.Metadata,
	}

	// Convert tool calls
	for _, tc := range cm.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: llm.ToolFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return msg
}

// convertCoreMsgsToLLMMsgs converts a slice of core.Message to llm.Message.
func convertCoreMsgsToLLMMsgs(msgs []core.Message) []llm.Message {
	result := make([]llm.Message, len(msgs))
	for i, cm := range msgs {
		result[i] = convertCoreMsgToLLMMsg(cm)
	}
	return result
}
