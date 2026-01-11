package orchestration

import (
	"context"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/shared"
	"time"
)

// Service provides transport-agnostic message orchestration.
// This service handles the core business logic of processing messages,
// managing threads, routing to agents, and coordinating execution.
// It is designed to be reusable across different transports (HTTP/SSE, A2A, etc.).
type Service struct {
	log                  *logger.Logger
	chatWriteService     *chatwrite.ChatWriteService
	threadStore          chatwrite.ThreadStore
	whiteboardProjection domain.WhiteboardStore
	eventLog             core.EventStore
	chatClient           chatwrite.ChatClient
}

// NewService creates a new orchestration service.
func NewService(
	log *logger.Logger,
	chatWriteService *chatwrite.ChatWriteService,
	threadStore chatwrite.ThreadStore,
	whiteboardProjection domain.WhiteboardStore,
	eventLog core.EventStore,
	chatClient chatwrite.ChatClient,
) *Service {
	return &Service{
		log:                  log,
		chatWriteService:     chatWriteService,
		threadStore:          threadStore,
		whiteboardProjection: whiteboardProjection,
		eventLog:             eventLog,
		chatClient:           chatClient,
	}
}

// UserMessageParams contains parameters for handling a user message.
type UserMessageParams struct {
	ThreadID  string
	MessageID string
	Content   string
	UserID    string
}

// HandleUserMessage processes an incoming user message and coordinates agent execution.
// This is the main entry point for user message processing.
// It is transport-agnostic and can be used from any handler (HTTP, A2A, etc.).
//
// The emitter parameter is a transport-agnostic event emitter that allows
// this service to emit events without knowing about the underlying transport.
func (s *Service) HandleUserMessage(
	ctx context.Context,
	params UserMessageParams,
	emitter EventEmitter,
) error {
	// Check if context was cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("task cancelled: %w", ctx.Err())
	}

	// Note: Thread creation is now handled automatically by AddUserMessage
	// if the thread doesn't exist (version 0)

	// 1. Add user message to thread (creates thread if needed)
	err := s.chatWriteService.AddUserMessage(ctx, params.ThreadID, params.MessageID, params.Content)
	if err != nil {
		s.log.Error(ctx, "Failed to add user message",
			"threadID", params.ThreadID,
			"error", err)
		return fmt.Errorf("failed to add user message: %w", err)
	}

	// 2. Generate unique run ID
	runID := domain.GenerateMessageID()

	// 3. Auto-generate thread title if this is the first message
	thread, threadExists := s.threadStore.Get(params.ThreadID)

	if threadExists && thread.Title == "New Chat" && len(thread.Messages) == 1 {
		title := params.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		err := s.chatWriteService.UpdateThreadTitle(ctx, params.ThreadID, title)
		if err != nil {
			s.log.Error(ctx, "Failed to update thread title",
				"threadID", params.ThreadID,
				"error", err)
		}

		// Emit domain event for thread title update
		titleEvent := &domain.ThreadTitleUpdatedEvent{
			ThreadID:  params.ThreadID,
			Title:     title,
			Timestamp: time.Now(),
		}
		if err := emitter.Emit(titleEvent); err != nil {
			s.log.Error(ctx, "Failed to emit thread title updated event",
				"threadID", params.ThreadID,
				"error", err)
		}
	}

	// 5. Run agent handoff chain starting with student-facing agent
	if err := emitter.Emit(&domain.RunStartedEvent{
		ThreadID:  params.ThreadID,
		RunID:     runID,
		Timestamp: time.Now(),
	}); err != nil {
		s.log.Error(ctx, "failed to emit run started event", "error", err, "threadID", params.ThreadID)
	}

	s.runAgentHandoffChain(ctx, params.ThreadID, "student-facing-agent", emitter, runID)

	if err := emitter.Emit(&domain.RunFinishedEvent{
		ThreadID:  params.ThreadID,
		RunID:     runID,
		Timestamp: time.Now(),
	}); err != nil {
		s.log.Error(ctx, "failed to emit run finished event", "error", err, "threadID", params.ThreadID)
	}

	return nil
}

// runAgentHandoffChain runs agents in sequence, following handoffs until completion.
func (s *Service) runAgentHandoffChain(ctx context.Context, threadID, startAgent string, emitter EventEmitter, runID string) {
	currentAgent := startAgent

	for currentAgent != "" {
		if ctx.Err() != nil {
			return
		}

		nextAgent, err := s.runAgent(ctx, threadID, currentAgent, emitter)
		if err != nil {
			if emitErr := emitter.Emit(&domain.RunErrorEvent{
				ThreadID:  threadID,
				Message:   fmt.Sprintf("Agent %s failed: %v", currentAgent, err),
				ErrorCode: "AGENT_RUN_ERROR",
				Timestamp: time.Now(),
			}); emitErr != nil {
				s.log.Error(ctx, "failed to emit run error event", "error", emitErr, "threadID", threadID)
			}
			return
		}

		currentAgent = nextAgent
	}
}

// runAgent runs a single agent and returns the next agent name if a handoff occurred.
func (s *Service) runAgent(ctx context.Context, threadID, agentName string, emitter EventEmitter) (string, error) {
	s.log.Info(ctx, "Executing agent", "threadID", threadID, "agent", agentName)

	// Get agent factory
	factory, ok := shared.GlobalAgents.Get(agentName)
	if !ok {
		return "", fmt.Errorf("agent %s not found", agentName)
	}

	// Create agent instance with event notifier
	notifier := NewUINotifier(emitter)
	agentConfig := factory(notifier, s.whiteboardProjection)

	var handoffReqCtx *core.RequestContext
	agent := chatwrite.New(
		s.log,
		s.chatClient,
		s.eventLog,
		s.threadStore,
		chatwrite.WithConfig(agentConfig),
		chatwrite.WithShouldContinueIteration(shared.CreateShouldContinueIterationFn()),
		chatwrite.WithRequestContextCapture(&handoffReqCtx),
	)

	// Get thread state
	thread, err := s.threadStore.GetOrCreateAgentThread(ctx, threadID)
	if err != nil {
		return "", fmt.Errorf("failed to get thread: %w", err)
	}

	// Debug: Log thread state
	messages := thread.GetMessages()
	s.log.Info(ctx, "Thread state before agent execution",
		"threadID", threadID,
		"messageCount", len(messages),
		"agentName", agentName)
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1]
		s.log.Info(ctx, "Last message in thread",
			"role", lastMsg.Role,
			"content", lastMsg.Content[:min(50, len(lastMsg.Content))])
	}

	// Execute agent
	messageID := domain.GenerateMessageID()
	if err := agent.RunWithThread(ctx, threadID, messageID, thread, core.DefaultRunOptions()); err != nil {
		return "", err
	}

	// Check for handoff to next agent
	if handoffReqCtx != nil {
		if handoff, ok := middleware.GetHandoff(handoffReqCtx); ok {
			s.log.Info(ctx, "Agent handoff", "from", agentName, "to", handoff.TargetAgent)
			return handoff.TargetAgent, nil
		}
	}

	return "", nil
}
