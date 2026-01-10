package orchestration

import (
	"context"
	"fmt"
	"log/slog"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/shared"
)

// Service provides transport-agnostic message orchestration.
// This service handles the core business logic of processing messages,
// managing threads, routing to agents, and coordinating execution.
// It is designed to be reusable across different transports (HTTP/SSE, A2A, etc.).
type Service struct {
	chatWriteService     *chatwrite.ChatWriteService
	threadStore          chatwrite.ThreadStore
	whiteboardProjection domain.WhiteboardStore
	eventLog             core.EventStore
	chatClient           chatwrite.ChatClient
}

// NewService creates a new orchestration service.
func NewService(
	chatWriteService *chatwrite.ChatWriteService,
	threadStore chatwrite.ThreadStore,
	whiteboardProjection domain.WhiteboardStore,
	eventLog core.EventStore,
	chatClient chatwrite.ChatClient,
) *Service {
	return &Service{
		chatWriteService:     chatWriteService,
		threadStore:          threadStore,
		whiteboardProjection: whiteboardProjection,
		eventLog:             eventLog,
		chatClient:           chatClient,
	}
}

// ProcessMessageParams contains parameters for message processing.
type ProcessMessageParams struct {
	ThreadID  string
	MessageID string
	Content   string
	UserID    string
}

// ProcessMessage handles core message processing logic.
// This is the main entry point for processing user messages.
// It is transport-agnostic and can be used from any handler (HTTP, A2A, etc.).
//
// The emitter parameter is a transport-agnostic event emitter that allows
// this service to emit events without knowing about the underlying transport.
func (s *Service) ProcessMessage(
	ctx context.Context,
	params ProcessMessageParams,
	emitter EventEmitter,
) error {
	// Check if context was cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("task cancelled: %w", ctx.Err())
	}

	// 1. Ensure thread exists, create if not
	_, exists := s.threadStore.Get(params.ThreadID)
	if !exists {
		err := s.chatWriteService.CreateThread(ctx, params.ThreadID, "New Chat")
		if err != nil {
			slog.Error("Failed to create thread",
				"threadID", params.ThreadID,
				"error", err)
			return fmt.Errorf("failed to create thread: %w", err)
		}
	}

	// 2. Add user message to thread
	err := s.chatWriteService.AddUserMessage(ctx, params.ThreadID, params.MessageID, params.Content)
	if err != nil {
		slog.Error("Failed to add user message",
			"threadID", params.ThreadID,
			"error", err)
		return fmt.Errorf("failed to add user message: %w", err)
	}

	// 3. Generate unique run ID
	runID := domain.GenerateMessageID()

	// 4. Emit run started event (domain event)
	runStartedEvent := &RunStartedEvent{
		ThreadID: params.ThreadID,
		RunID:    runID,
	}
	if err := emitter.Emit(runStartedEvent); err != nil {
		slog.Error("Failed to emit run started event",
			"threadID", params.ThreadID,
			"error", err)
	}

	// 5. Auto-generate thread title if this is the first message
	thread, threadExists := s.threadStore.Get(params.ThreadID)
	if threadExists && thread.Title == "New Chat" && len(thread.Messages) == 1 {
		title := params.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		err := s.chatWriteService.UpdateThreadTitle(ctx, params.ThreadID, title)
		if err != nil {
			slog.Error("Failed to update thread title",
				"threadID", params.ThreadID,
				"error", err)
		}

		// Emit domain event for thread title update
		titleEvent := &ThreadTitleUpdatedEvent{
			ThreadID: params.ThreadID,
			Title:    title,
		}
		if err := emitter.Emit(titleEvent); err != nil {
			slog.Error("Failed to emit thread title updated event",
				"threadID", params.ThreadID,
				"error", err)
		}
	}

	// 6. Execute student-facing agent (entry point)
	s.executeAgent(ctx, params.ThreadID, "student-facing-agent", emitter, runID)

	// 7. Emit run finished event (domain event)
	runFinishedEvent := &RunFinishedEvent{
		ThreadID: params.ThreadID,
		RunID:    runID,
	}
	if err := emitter.Emit(runFinishedEvent); err != nil {
		slog.Error("Failed to emit run finished event",
			"threadID", params.ThreadID,
			"error", err)
	}

	return nil
}

// executeAgent runs a specific agent and handles handoffs.
// This method is recursive to support agent-to-agent handoffs.
func (s *Service) executeAgent(
	ctx context.Context,
	threadID string,
	agentName string,
	emitter EventEmitter,
	runID string,
) {
	// Check if context was cancelled before starting agent
	if ctx.Err() != nil {
		slog.Warn("Agent execution cancelled",
			"threadID", threadID,
			"agent", agentName)
		return
	}

	slog.Info("Executing agent",
		"threadID", threadID,
		"agent", agentName)

	// Create UI notifier for agent SDK events
	notifier := NewUINotifier(emitter)

	// Get the agent's factory
	factory, ok := shared.GlobalAgents.Get(agentName)
	if !ok {
		slog.Error("No factory found for agent", "agent", agentName)
		return
	}

	// Create agent config from factory
	agentConfig := factory(notifier, s.whiteboardProjection)

	// Create agent instance
	var handoffReqCtx *core.RequestContext
	agentInstance := chatwrite.New(
		s.chatClient,
		s.eventLog,
		s.threadStore,
		chatwrite.WithConfig(agentConfig),
		chatwrite.WithShouldContinueIteration(shared.CreateShouldContinueIterationFn()),
		chatwrite.WithRequestContextCapture(&handoffReqCtx),
	)

	// Get or create the thread (for new threads, this creates an empty one)
	agentThread, err := s.threadStore.GetOrCreateAgentThread(ctx, threadID)
	if err != nil {
		slog.Error("Failed to get/create thread",
			"threadID", threadID,
			"error", err)
		return
	}

	// Generate message ID for this run
	messageID := domain.GenerateMessageID()

	// Run the agent
	runOpts := core.DefaultRunOptions()
	err = agentInstance.RunWithThread(ctx, threadID, messageID, agentThread, runOpts)
	if err != nil {
		slog.Error("Agent run failed",
			"agent", agentName,
			"threadID", threadID,
			"error", err)

		// Emit domain error event
		errorEvent := &RunErrorEvent{
			Message:   fmt.Sprintf("Agent %s failed: %v", agentName, err),
			ErrorCode: "AGENT_RUN_ERROR",
		}
		_ = emitter.Emit(errorEvent)

		return
	}

	slog.Info("Agent run completed",
		"agent", agentName,
		"threadID", threadID)

	// Check for agent handoff
	if handoffReqCtx != nil {
		if handoff, ok := middleware.GetHandoff(handoffReqCtx); ok {
			slog.Info("Processing agent handoff",
				"threadID", threadID,
				"from", agentName,
				"to", handoff.TargetAgent,
				"question", handoff.Question)

			// Continue with target agent (recursive)
			s.executeAgent(ctx, threadID, handoff.TargetAgent, emitter, runID)
		}
	}
}
