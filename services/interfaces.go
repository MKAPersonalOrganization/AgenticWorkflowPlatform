package services

import (
	"context"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// AuthService provides authentication and authorization operations.
type AuthService interface {
	// GetUserFromContext extracts authenticated user from request context
	GetUserFromContext(ctx context.Context) *domain.User
	// IsAuthenticated checks if request context has valid authentication
	IsAuthenticated(ctx context.Context) bool
}

// ThreadQueryService provides read access to projections (queries) for UI.
type ThreadQueryService interface {
	// GetThreadMessages retrieves AGUI-formatted messages for a thread
	GetThreadMessages(threadID string) []aguievents.Message
	// ListThreadSummaries retrieves all threads with summaries, sorted by most recent
	ListThreadSummaries() []ThreadSummary
}

// AgentOrchestrator defines the interface for running agent-based workflows.
type AgentOrchestrator interface {
	// Run processes the LLM response using an AgentThread and yields events.
	Run(ctx context.Context, agentName string, runCtx core.RunContext) iter.Seq2[core.Event, error]

	// ProcessUserMessage handles a new user message from start to finish.
	ProcessUserMessage(ctx context.Context, threadID, content, userID string) (iter.Seq2[core.Event, error], error)
}

// AgentFactory creates instances of AgentOrchestrator for specific agents.
type AgentFactory interface {
	// Create creates a new AgentOrchestrator with the given configuration and options.
	Create(config core.AgentConfig, reqCtxCapture **core.RequestContext, toolTracer core.ToolTracer) AgentOrchestrator
}

// ThreadSummary represents a summary of a thread for list views.
type ThreadSummary struct {
	ID                string               `json:"id"`
	Title             string               `json:"title"`
	Messages          []aguievents.Message `json:"messages"`
	CreatedAt         string               `json:"createdAt"`
	UpdatedAt         string               `json:"updatedAt"`
	CurrentInstructor string               `json:"currentInstructor,omitempty"`
}
