package services

import (
	"context"
	"school-with-ai/domain"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// AuthService provides authentication and authorization operations.
type AuthService interface {
	// GetUserFromContext extracts authenticated user from request context
	GetUserFromContext(ctx context.Context) *domain.User
	// IsAuthenticated checks if request context has valid authentication
	IsAuthenticated(ctx context.Context) bool
}

// ProjectionService provides read access to projections.
type ProjectionService interface {
	// GetWhiteboardContent retrieves whiteboard content for a thread
	GetWhiteboardContent(threadID string) string
	// GetThreadHistory retrieves formatted AGUI message history
	GetThreadHistory(threadID string) (any, error)
	// GetThread retrieves a thread by ID
	GetThread(threadID string) (*domain.Thread, bool)
	// GetAllThreads retrieves all threads
	GetAllThreads() []domain.Thread
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
	WhiteboardContent string               `json:"whiteboardContent"`
	CurrentInstructor string               `json:"currentInstructor,omitempty"`
}
