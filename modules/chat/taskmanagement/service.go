package taskmanagement

import (
	"context"
	"fmt"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/agentorchestrator"
)

// Orchestrator is the interface for orchestrating agent runs with lifecycle events.
type Orchestrator interface {
	ExecuteRun(ctx context.Context, req agentorchestrator.RunExecutionRequest) iter.Seq2[domain.Event, error]
}

// Service provides transport-agnostic message taskmanagement.
// This service handles the core business logic of processing messages,
// managing threads, routing to agents, and coordinating execution.
// It is designed to be reusable across different transports (HTTP/SSE, A2A, etc.).
type Service struct {
	log          *logger.Logger
	orchestrator Orchestrator
}

// NewService creates a new orchestration service.
func NewService(
	log *logger.Logger,
	orchestrator Orchestrator,
) *Service {
	return &Service{
		log:          log,
		orchestrator: orchestrator,
	}
}

// HandleUserMessage processes an incoming user message and coordinates agent execution.
// This is the main entry point for user message processing.
// It is transport-agnostic and can be used from any handler (HTTP, A2A, etc.).
func (s *Service) HandleUserMessage(
	ctx context.Context,
	req agentorchestrator.RunExecutionRequest,
) error {
	// Check if context was cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("task cancelled: %w", ctx.Err())
	}

	// Delegate to Orchestrator (which emits RunStarted/RunFinished/RunError)
	for _, err := range s.orchestrator.ExecuteRun(ctx, req) {
		if err != nil {
			return err
		}
	}

	return nil
}
