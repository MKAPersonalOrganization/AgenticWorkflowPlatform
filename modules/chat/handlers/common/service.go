package common

import (
	"aicompass/modules/chat/agentorchestrator"
	"aicompass/modules/chat/thread"
	"context"
	"iter"
)

// Orchestrator is the interface for orchestrating agent runs with lifecycle events.
type Orchestrator interface {
	ExecuteRun(ctx context.Context, req agentorchestrator.RunExecutionRequest) iter.Seq2[thread.Event, error]
}
