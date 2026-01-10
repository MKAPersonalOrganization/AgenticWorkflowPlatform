package chatwrite

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"school-with-ai/domain"
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
	config                  core.AgentConfig
	chatClient              ChatClient
	writeModel              core.EventStore
	readModel               ThreadStore
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
func New(chatClient ChatClient, writeModel core.EventStore, readModel ThreadStore, opts ...Option) *ChatWriteService {
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

			// Persist ToolCallExecuted event to event store (for projections like whiteboard)
			err = h.addToolCallExecutedEvent(ctx, threadID, toolResultID, toolCall, messageID, result)
			if err != nil {
				return fmt.Errorf("failed to add tool call executed event: %w", err)
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
	fmt.Printf("[ChatWrite] CreateThread called: threadID=%s, title=%s\n", threadID, title)

	// Create repository and decider
	repo := domain.NewThreadRepository(h.writeModel)
	decider := domain.NewThreadDecider()

	// Try to load existing thread first (idempotency check)
	existingThread, err := repo.Load(ctx, threadID)
	if err == nil && existingThread.Version > 0 {
		fmt.Printf("[ChatWrite] Thread already exists (version=%d), skipping create\n", existingThread.Version)
		// Thread already exists, this is idempotent
		return nil
	}
	fmt.Printf("[ChatWrite] Thread does not exist, creating new thread\n")

	fmt.Printf("[ChatWrite] Thread does not exist, creating new thread\n")

	// Create new aggregate
	thread := domain.NewThreadAggregate(threadID)

	// Create command
	cmd := domain.CreateThread{
		ThreadID: threadID,
		UserID:   "system", // TODO: Get from context
		Title:    title,
	}
	fmt.Printf("[ChatWrite] CreateThread command: %+v\n", cmd)

	// Execute command through decider
	events, err := decider.Decide(cmd, thread)
	if err != nil {
		// If thread already exists, ignore (idempotent)
		if err.Error() == "thread already exists" {
			fmt.Printf("[ChatWrite] Decider returned 'thread already exists', skipping\n")
			return nil
		}
		return fmt.Errorf("failed to decide create thread: %w", err)
	}
	fmt.Printf("[ChatWrite] Decider returned %d events\n", len(events))

	// Enqueue events and apply to in-memory state
	for i, event := range events {
		fmt.Printf("[ChatWrite] Event %d: %T - %s\n", i, event, event.EventType())
		thread.Enqueue(event)
		// Evolve for in-memory state
		thread.Evolve(event)
	}

	fmt.Printf("[ChatWrite] Saving new thread with %d events\n", len(events))
	err = repo.SaveNew(ctx, thread)
	if err != nil {
		fmt.Printf("[ChatWrite] SaveNew failed: %v\n", err)
		// If concurrency conflict, thread was created by another request - this is fine
		if errors.Is(err, core.ErrConcurrencyConflict) {
			fmt.Printf("[ChatWrite] Concurrency conflict, thread already created\n")
			return nil
		}
		return fmt.Errorf("failed to save new thread: %w", err)
	}
	fmt.Printf("[ChatWrite] Thread created successfully: %s\n", threadID)

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
	fmt.Printf("[ChatWrite] AddUserMessage called: threadID=%s, messageID=%s\n", threadID, messageID)

	// Create repository and decider
	repo := domain.NewThreadRepository(h.writeModel)
	decider := domain.NewThreadDecider()

	// Load existing aggregate
	thread, err := repo.Load(ctx, threadID)
	if err != nil {
		fmt.Printf("[ChatWrite] Failed to load thread for message: %v\n", err)
		return fmt.Errorf("failed to load thread: %w", err)
	}

	fmt.Printf("[ChatWrite] Thread loaded for message: version=%d, title=%q\n", thread.Version, thread.Title)

	// Remember if this is a new thread (for choosing Save vs SaveNew)
	isNewThread := thread.Version == 0

	// If this is a new thread (version 0), create it first with a default title
	if isNewThread {
		fmt.Printf("[ChatWrite] Thread is new (version 0), creating with ThreadCreated event first\n")

		createCmd := domain.CreateThread{
			ThreadID: threadID,
			UserID:   "system", // TODO: Get from context
			Title:    "New Chat",
		}

		createEvents, err := decider.Decide(createCmd, thread)
		if err != nil {
			fmt.Printf("[ChatWrite] Failed to create thread in AddUserMessage: %v\n", err)
			return fmt.Errorf("failed to create thread: %w", err)
		}

		fmt.Printf("[ChatWrite] ThreadCreated event generated: %d events\n", len(createEvents))
		for i, event := range createEvents {
			fmt.Printf("[ChatWrite] Create event %d: %T - %s, aggregateID=%q\n",
				i, event, event.EventType(), event.AggregateID())
			thread.Enqueue(event)
			// Evolve for in-memory state so the next Decide sees the thread as created
			thread.Evolve(event)
		}
		fmt.Printf("[ChatWrite] After applying ThreadCreated: title=%q, ID=%q, version=%d\n",
			thread.Title, thread.ID, thread.Version)
	}

	// Create command for adding the message
	cmd := domain.AddMessage{
		ThreadID:  threadID,
		MessageID: messageID,
		Role:      "user",
		Content:   content,
	}

	// Execute command through decider
	events, err := decider.Decide(cmd, thread)
	if err != nil {
		fmt.Printf("[ChatWrite] Failed to decide add message: %v\n", err)
		return fmt.Errorf("failed to decide add message: %w", err)
	}

	fmt.Printf("[ChatWrite] AddMessage decider returned %d events\n", len(events))

	// Enqueue events and apply to in-memory state
	for i, event := range events {
		fmt.Printf("[ChatWrite] Message event %d: %T - %s\n", i, event, event.EventType())
		thread.Enqueue(event)
		// Evolve for in-memory state
		thread.Evolve(event)
	}

	// Save the thread
	// For new threads: use SaveNew (expects version 0 in event store)
	// For existing threads: use Save (expects current version)
	if isNewThread {
		fmt.Printf("[ChatWrite] New thread - using SaveNew (expects empty stream)\n")
		fmt.Printf("[ChatWrite] Thread state before save: version=%d, uncommitted events=%d\n",
			thread.Version, thread.UncommittedEventsCount())

		err = repo.SaveNew(ctx, thread)
		if err != nil {
			fmt.Printf("[ChatWrite] SaveNew failed: %v\n", err)
			// If concurrency conflict, thread was created by another request
			if errors.Is(err, core.ErrConcurrencyConflict) {
				fmt.Printf("[ChatWrite] Concurrency conflict - thread created by another request\n")
				return nil // Idempotent - thread exists, no error
			}
			return fmt.Errorf("failed to save new thread: %w", err)
		}
	} else {
		fmt.Printf("[ChatWrite] Existing thread - using Save with version check\n")
		err = repo.Save(ctx, thread)
		if err != nil {
			fmt.Printf("[ChatWrite] Failed to save thread: %v\n", err)
			return fmt.Errorf("failed to save thread: %w", err)
		}
	}

	fmt.Printf("[ChatWrite] Thread saved successfully\n")

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
	repo := domain.NewThreadRepository(h.writeModel)
	decider := domain.NewThreadDecider()

	thread, err := repo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	cmd := domain.AddMessage{
		ThreadID:  threadID,
		MessageID: messageID,
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
		AgentName: h.config.Name,
	}

	events, err := decider.Decide(cmd, thread)
	if err != nil {
		return fmt.Errorf("failed to decide add assistant message: %w", err)
	}

	for _, event := range events {
		thread.Enqueue(event)
		// Evolve for in-memory state
		thread.Evolve(event)
	}

	return repo.Save(ctx, thread)
}

// addToolResultMessage adds a tool result message to the thread using event sourcing.
func (h *ChatWriteService) addToolResultMessage(ctx context.Context, threadID, messageID, toolCallID, result string) error {
	repo := domain.NewThreadRepository(h.writeModel)
	decider := domain.NewThreadDecider()

	thread, err := repo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	cmd := domain.AddMessage{
		ThreadID:   threadID,
		MessageID:  messageID,
		Role:       "tool",
		Content:    result,
		ToolCallID: toolCallID,
		AgentName:  h.config.Name,
	}

	events, err := decider.Decide(cmd, thread)
	if err != nil {
		return fmt.Errorf("failed to decide add tool result: %w", err)
	}

	for _, event := range events {
		thread.Enqueue(event)
		// Evolve for in-memory state
		thread.Evolve(event)
	}

	return repo.Save(ctx, thread)
}

// addToolCallExecutedEvent persists a ToolCallExecuted event to enable projection rebuilding.
func (h *ChatWriteService) addToolCallExecutedEvent(ctx context.Context, threadID, messageID string, toolCall core.ToolCall, parentMessageID, result string) error {
	repo := domain.NewThreadRepository(h.writeModel)

	thread, err := repo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	event := &domain.ToolCallExecuted{
		ThreadID:        threadID,
		MessageID:       messageID,
		ToolCallID:      toolCall.ID,
		ToolName:        toolCall.Function.Name,
		ToolArguments:   toolCall.Function.Arguments,
		ToolResult:      result,
		ParentMessageID: parentMessageID,
		AgentName:       h.config.Name,
		Timestamp:       time.Now(),
	}

	thread.Enqueue(event)
	thread.Evolve(event)

	return repo.Save(ctx, thread)
}

// UpdateThreadTitle updates the title of a thread using event sourcing.
func (h *ChatWriteService) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) error {
	fmt.Printf("[ChatWrite] UpdateThreadTitle called: threadID=%s, newTitle=%s\n", threadID, newTitle)

	// Create repository and decider
	repo := domain.NewThreadRepository(h.writeModel)
	decider := domain.NewThreadDecider()

	// Load existing aggregate
	thread, err := repo.Load(ctx, threadID)
	if err != nil {
		fmt.Printf("[ChatWrite] Failed to load thread for title update: %v\n", err)
		return fmt.Errorf("failed to load thread: %w", err)
	}
	fmt.Printf("[ChatWrite] Thread loaded: version=%d, currentTitle=%s\n", thread.Version, thread.Title)

	// Create command
	cmd := domain.ChangeThreadTitle{
		ThreadID: threadID,
		NewTitle: newTitle,
	}

	// Execute command through decider
	events, err := decider.Decide(cmd, thread)
	if err != nil {
		fmt.Printf("[ChatWrite] Decider failed for title change: %v\n", err)
		return fmt.Errorf("failed to decide change title: %w", err)
	}
	fmt.Printf("[ChatWrite] Decider returned %d events for title change\n", len(events))

	// Enqueue events and apply to in-memory state
	for i, event := range events {
		fmt.Printf("[ChatWrite] Title change event %d: %T - %s\n", i, event, event.EventType())
		thread.Enqueue(event)
		// Evolve for in-memory state
		thread.Evolve(event)
	}

	err = repo.Save(ctx, thread)
	if err != nil {
		fmt.Printf("[ChatWrite] Failed to save thread after title change: %v\n", err)
		return fmt.Errorf("failed to save thread: %w", err)
	}
	fmt.Printf("[ChatWrite] Thread title updated successfully: %s -> %s\n", thread.Title, newTitle)

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
	repo := domain.NewThreadRepository(h.writeModel)
	err := repo.Delete(ctx, threadID)
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
