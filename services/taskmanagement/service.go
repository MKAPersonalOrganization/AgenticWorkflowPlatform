package taskmanagement

import (
	"context"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/observability/langfuse"
	"school-with-ai/services"
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
	log                 *logger.Logger
	orchestratorService services.AgentOrchestrator
	agentFactory        services.AgentFactory
	threadStore         domain.ThreadStore
	eventLog            core.EventStore
	chatClient          ChatClient
	langfuseTracer      *LangfuseTracer
	observabilityConfig ObservabilityConfig
}

// NewService creates a new orchestration service.
func NewService(
	log *logger.Logger,
	orchestratorService services.AgentOrchestrator,
	agentFactory services.AgentFactory,
	threadStore domain.ThreadStore,
	eventLog core.EventStore,
	chatClient ChatClient,
	langfuseTracer *LangfuseTracer,
	observabilityConfig ObservabilityConfig,
) *Service {
	return &Service{
		log:                 log,
		orchestratorService: orchestratorService,
		agentFactory:        agentFactory,
		threadStore:         threadStore,
		eventLog:            eventLog,
		chatClient:          chatClient,
		langfuseTracer:      langfuseTracer,
		observabilityConfig: observabilityConfig,
	}
}

// UserMessageParams contains parameters for handling a user message.
type UserMessageParams struct {
	ThreadID  string
	MessageID string
	Content   string
	UserID    string
	AgentName string
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
) error {
	// Check if context was cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("task cancelled: %w", ctx.Err())
	}

	// 1. Start Langfuse trace (if enabled)
	var traceCtx *langfuse.TraceContext
	if s.langfuseTracer != nil {
		tracer := s.langfuseTracer.GetTracer()
		if tracer != nil {
			tc, err := tracer.StartTrace(ctx, "user-message",
				langfuse.WithTraceUserID(params.UserID),
				langfuse.WithTraceSessionID(params.ThreadID),
				langfuse.WithTraceMetadata(map[string]any{
					"messageId": params.MessageID,
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

	if traceCtx != nil {
		ctx = core.WithTraceContext(ctx, traceCtx.TraceID, traceCtx.ParentObservationID)
	}

	// 2. Delegate to AgentOrchestrator
	events, err := s.orchestratorService.ProcessUserMessage(ctx, params.AgentName, params.ThreadID, params.Content, params.UserID)
	if err != nil {
		return fmt.Errorf("failed to process user message: %w", err)
	}

	// 3. Persist ephemeral events to EventStore
	// This ensures that streaming events (processed by ProjectionUpdater) reach the AguiProjection
	for event, err := range events {
		if err != nil {
			return fmt.Errorf("agent run failed: %w", err)
		}

		// Filter out events that were already persisted by the Engine/ThreadMgmt to avoid duplication
		switch event.EventType() {
		case "MessageAdded", "ThreadCreated", "ThreadTitleUpdated", "ThreadDeleted":
			// These are handled by threadMgmt internal calls
			continue
		default:
			// Write ephemeral events (streaming tokens, run status, etc.) to EventStore
			// We use a specific streaming stream to avoid optimistic concurrency conflicts with the main thread stream
			streamID := fmt.Sprintf("streaming-%s", params.ThreadID)
			if _, err := s.eventLog.AppendWithVersion(ctx, streamID, -1, event); err != nil {
				s.log.Error(ctx, "failed to append ephemeral event", "error", err, "type", event.EventType())
			}
		}
	}

	return nil
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
