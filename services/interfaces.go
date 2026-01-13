package services

import (
	"context"
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

// ProjectionService provides read access to projections for UI.
type ProjectionService interface {
	// GetThreadMessages retrieves AGUI-formatted messages for a thread
	GetThreadMessages(threadID string) []aguievents.Message
	// ListThreadSummaries retrieves all threads with summaries, sorted by most recent
	ListThreadSummaries() []ThreadSummary
}

// AgentOrchestrator defines the interface for running agent-based workflows.
type AgentOrchestrator interface {
	// RunWithThread processes the LLM response using an AgentThread
	RunWithThread(ctx context.Context, threadID, messageID string, thread core.AgentThread, runOpts core.RunOptions) error
	// AddUserMessage adds a user message to a thread using event sourcing
	AddUserMessage(ctx context.Context, threadID, messageID, content string) (core.AgentThread, error)
	// CreateThread creates a new chat thread using event sourcing
	CreateThread(ctx context.Context, threadID, title string) (core.AgentThread, error)
	// UpdateThreadTitle updates the title of a thread using event sourcing
	UpdateThreadTitle(ctx context.Context, threadID, newTitle string) error
	// DeleteThread deletes a thread using event sourcing
	DeleteThread(ctx context.Context, threadID string) error
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
