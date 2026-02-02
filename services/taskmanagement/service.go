package taskmanagement

import (
	"context"
	"fmt"
	"iter"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/observability/langfuse"
	"school-with-ai/services/agentorchestrator"
)

// ObservabilityConfig contains configuration for observability and tracing attributes
type ObservabilityConfig struct {
	Release    string   // Release version (e.g., "v1.2.3")
	Env        string   // Environment (e.g., "production", "staging", "development")
	Deployment string   // Deployment context (e.g., "us-east-1", "cluster-a")
	Tags       []string // Additional custom tags
}

// Orchestrator is the interface for orchestrating agent runs with lifecycle events.
type Orchestrator interface {
	ExecuteRun(ctx context.Context, req agentorchestrator.RunExecutionRequest) iter.Seq2[domain.Event, error]
}

// Service provides transport-agnostic message taskmanagement.
// This service handles the core business logic of processing messages,
// managing threads, routing to agents, and coordinating execution.
// It is designed to be reusable across different transports (HTTP/SSE, A2A, etc.).
type Service struct {
	log                 *logger.Logger
	orchestrator        Orchestrator
	model               agent.Model
	langfuseTracer      *LangfuseTracer
	observabilityConfig ObservabilityConfig
}

// NewService creates a new orchestration service.
func NewService(
	log *logger.Logger,
	orchestrator Orchestrator,
	model agent.Model,
	langfuseTracer *LangfuseTracer,
	observabilityConfig ObservabilityConfig,
) *Service {
	return &Service{
		log:                 log,
		orchestrator:        orchestrator,
		model:               model,
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
		ctx = agent.WithTraceContext(ctx, traceCtx.TraceID, traceCtx.ParentObservationID)
	}

	// 2. Delegate to Orchestrator (which emits RunStarted/RunFinished/RunError)
	req := agentorchestrator.RunExecutionRequest{
		SessionID:   params.ThreadID,
		UserID:      params.UserID,
		AgentName:   params.AgentName,
		Message:     params.Content,
		MessageID:   params.MessageID,
		ParentRunID: "", // TODO: Support branching/time travel
	}

	// Iterate over orchestrated events (includes RunStarted, framework events, RunFinished/RunError)
	for _, err := range s.orchestrator.ExecuteRun(ctx, req) {
		if err != nil {
			return err
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
