package taskmanagement

import (
	"context"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/observability/langfuse"
	"school-with-ai/services/agentorchestrator"
	"school-with-ai/services/agentorchestrator/agents/shared"
	"time"
)

// ObservabilityConfig contains configuration for observability and tracing attributes
type ObservabilityConfig struct {
	Release    string   // Release version (e.g., "v1.2.3")
	Env        string   // Environment (e.g., "production", "staging", "development")
	Deployment string   // Deployment context (e.g., "us-east-1", "cluster-a")
	Tags       []string // Additional custom tags
}

// Service provides transport-agnostic message taskmanagement.
// This service handles the core business logic of processing messages,
// managing threads, routing to agents, and coordinating execution.
// It is designed to be reusable across different transports (HTTP/SSE, A2A, etc.).
type Service struct {
	log                  *logger.Logger
	orchestratorService  *agentorchestrator.Service
	threadStore          agentorchestrator.ThreadStore
	whiteboardProjection domain.WhiteboardStore
	eventLog             core.EventStore
	chatClient           agentorchestrator.ChatClient
	langfuseTracer       *LangfuseTracer
	observabilityConfig  ObservabilityConfig
}

// NewService creates a new orchestration service.
func NewService(
	log *logger.Logger,
	orchestratorService *agentorchestrator.Service,
	threadStore agentorchestrator.ThreadStore,
	whiteboardProjection domain.WhiteboardStore,
	eventLog core.EventStore,
	chatClient agentorchestrator.ChatClient,
	langfuseTracer *LangfuseTracer,
	observabilityConfig ObservabilityConfig,
) *Service {
	return &Service{
		log:                  log,
		orchestratorService:  orchestratorService,
		threadStore:          threadStore,
		whiteboardProjection: whiteboardProjection,
		eventLog:             eventLog,
		chatClient:           chatClient,
		langfuseTracer:       langfuseTracer,
		observabilityConfig:  observabilityConfig,
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
	err := s.orchestratorService.AddUserMessage(ctx, params.ThreadID, params.MessageID, params.Content)
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

		err := s.orchestratorService.UpdateThreadTitle(ctx, params.ThreadID, title)
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

	// 5. Start Langfuse trace (if enabled)
	var traceCtx *langfuse.TraceContext
	if s.langfuseTracer != nil {
		tracer := s.langfuseTracer.GetTracer()
		if tracer != nil {
			tc, err := tracer.StartTrace(ctx, "user-message",
				langfuse.WithTraceUserID(params.UserID),
				langfuse.WithTraceSessionID(params.ThreadID),
				langfuse.WithTraceMetadata(map[string]any{
					"messageId": params.MessageID,
					"runId":     runID,
				}),
				langfuse.WithTraceTags(s.getTraceTags()),
				langfuse.WithTraceRelease(s.getRelease()),
			)
			if err != nil {
				s.log.Warn(ctx, "failed to start Langfuse trace", "error", err)
			} else {
				traceCtx = tc
			}
		}
	}

	// 6. Run agent handoff chain starting with student-facing agent
	if err := emitter.Emit(&domain.RunStartedEvent{
		ThreadID:  params.ThreadID,
		RunID:     runID,
		Timestamp: time.Now(),
	}); err != nil {
		s.log.Error(ctx, "failed to emit run started event", "error", err, "threadID", params.ThreadID)
	}

	s.runAgentHandoffChain(ctx, params.ThreadID, "student-facing-agent", emitter, runID, traceCtx)

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
func (s *Service) runAgentHandoffChain(ctx context.Context, threadID, startAgent string, emitter EventEmitter, runID string, traceCtx *langfuse.TraceContext) {
	currentAgent := startAgent

	for currentAgent != "" {
		if ctx.Err() != nil {
			return
		}

		nextAgent, err := s.runAgent(ctx, threadID, currentAgent, emitter, traceCtx)
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
func (s *Service) runAgent(ctx context.Context, threadID, agentName string, emitter EventEmitter, traceCtx *langfuse.TraceContext) (string, error) {
	s.log.Info(ctx, "Executing agent", "threadID", threadID, "agent", agentName)

	// Start generation tracking for this agent (if Langfuse is enabled)
	var genTracker *langfuse.GenerationTracker
	if s.langfuseTracer != nil && traceCtx != nil {
		tracer := s.langfuseTracer.GetTracer()
		if tracer != nil {
			// Start generation for this agent
			tracker, err := tracer.StartGeneration(ctx, traceCtx, agentName, "gpt-4o",
				langfuse.WithGenerationMetadata(map[string]any{
					"threadId": threadID,
				}),
			)
			if err != nil {
				s.log.Warn(ctx, "failed to start generation tracking", "agent", agentName, "error", err)
			} else {
				genTracker = tracker
				// Update trace context to nest child observations under this generation
				traceCtx = &langfuse.TraceContext{
					TraceID:             traceCtx.TraceID,
					ParentObservationID: genTracker.GetObservationID(),
				}
			}
		}
	}

	// Get agent factory
	factory, ok := shared.GlobalAgents.Get(agentName)
	if !ok {
		return "", fmt.Errorf("agent %s not found", agentName)
	}

	// Create agent instance with event notifier
	notifier := NewUINotifier(emitter)
	agentConfig := factory(notifier, s.whiteboardProjection)

	var handoffReqCtx *core.RequestContext
	agentOpts := []agentorchestrator.Option{
		agentorchestrator.WithConfig(agentConfig),
		agentorchestrator.WithShouldContinueIteration(shared.CreateShouldContinueIterationFn()),
		agentorchestrator.WithRequestContextCapture(&handoffReqCtx),
	}

	// Add tool tracer if Langfuse is enabled
	if s.langfuseTracer != nil && traceCtx != nil {
		tracer := s.langfuseTracer.GetTracer()
		if tracer != nil {
			toolTracer := langfuse.NewLangfuseToolTracer(tracer)
			agentOpts = append(agentOpts, agentorchestrator.WithToolTracer(toolTracer))
		}
	}

	agent := agentorchestrator.New(
		s.log,
		s.chatClient,
		s.eventLog,
		s.threadStore,
		agentOpts...,
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

	// Add trace context to ctx for tool tracking (if enabled)
	if traceCtx != nil {
		ctx = core.WithTraceContext(ctx, traceCtx.TraceID, traceCtx.ParentObservationID)
	}

	// Execute agent
	messageID := domain.GenerateMessageID()
	agentErr := agent.RunWithThread(ctx, threadID, messageID, thread, core.DefaultRunOptions())

	// Complete generation tracking (if enabled)
	if genTracker != nil {
		if agentErr != nil {
			// Agent execution failed
			if err := genTracker.EndWithError(ctx, agentErr); err != nil {
				s.log.Warn(ctx, "failed to complete generation tracking with error", "agent", agentName, "error", err)
			}
		} else {
			// Agent execution succeeded
			// Get the agent's response from the thread
			updatedThread, _ := s.threadStore.GetOrCreateAgentThread(ctx, threadID)
			var response string
			if updatedMessages := updatedThread.GetMessages(); len(updatedMessages) > 0 {
				lastMsg := updatedMessages[len(updatedMessages)-1]
				if lastMsg.Role == "assistant" {
					response = lastMsg.Content
				}
			}

			if err := genTracker.End(ctx, map[string]any{
				"response": response,
			}, nil); err != nil { // Note: We don't have token usage info here
				s.log.Warn(ctx, "failed to complete generation tracking", "agent", agentName, "error", err)
			}
		}
	}

	if agentErr != nil {
		return "", agentErr
	}

	// Check for handoff to next agent
	if handoffReqCtx != nil {
		if handoff, ok := middleware.GetHandoff(handoffReqCtx); ok {
			s.log.Info(ctx, "Agent handoff", "from", agentName, "to", handoff.TargetAgent)

			// Log handoff event in Langfuse
			if s.langfuseTracer != nil && traceCtx != nil {
				tracer := s.langfuseTracer.GetTracer()
				if tracer != nil {
					_ = tracer.LogEvent(ctx, traceCtx, "agent-handoff",
						langfuse.WithEventInput(map[string]any{
							"from": agentName,
							"to":   handoff.TargetAgent,
						}),
					)
				}
			}

			return handoff.TargetAgent, nil
		}
	}

	return "", nil
}

// getRelease returns the application release version from config
func (s *Service) getRelease() string {
	// Use configured release if available
	if s.observabilityConfig.Release != "" {
		return s.observabilityConfig.Release
	}

	// Final fallback
	return "dev"
}

// getTraceTags returns configured tags for the trace
func (s *Service) getTraceTags() []string {
	tags := []string{}

	// Add environment tag
	if s.observabilityConfig.Env != "" {
		tags = append(tags, "env:"+s.observabilityConfig.Env)
	}

	// Add deployment context if available
	if s.observabilityConfig.Deployment != "" {
		tags = append(tags, "deployment:"+s.observabilityConfig.Deployment)
	}

	// Add custom tags from config
	tags = append(tags, s.observabilityConfig.Tags...)

	return tags
}
