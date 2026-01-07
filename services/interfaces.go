package services

import (
	"context"

	"school-with-ai/domain"
)

// AuthService provides authentication and authorization operations
type AuthService interface {
	// GetUserFromContext extracts authenticated user from request context
	GetUserFromContext(ctx context.Context) *domain.User
	// IsAuthenticated checks if request context has valid authentication
	IsAuthenticated(ctx context.Context) bool
}

// ProjectionService provides read access to projections
type ProjectionService interface {
	// GetWhiteboardContent retrieves whiteboard content for a thread
	GetWhiteboardContent(threadID string) string
	// GetThreadHistory retrieves formatted AGUI message history
	GetThreadHistory(threadID string) (interface{}, error)
}
