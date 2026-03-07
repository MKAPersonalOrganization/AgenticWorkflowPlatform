package services

import (
	"context"
	"school-with-ai/shared"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// AuthService provides authentication and authorization operations.
type AuthService interface {
	// GetUserFromContext extracts authenticated user from request context
	GetUserFromContext(ctx context.Context) *shared.User
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

// ThreadSummary represents a summary of a thread for list views.
type ThreadSummary struct {
	ID                string               `json:"id"`
	Title             string               `json:"title"`
	Messages          []aguievents.Message `json:"messages"`
	CreatedAt         string               `json:"createdAt"`
	UpdatedAt         string               `json:"updatedAt"`
	CurrentInstructor string               `json:"currentInstructor,omitempty"`
}
