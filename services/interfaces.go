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

// JobService manages job lifecycle and queue operations
type JobService interface {
	// SubmitJob creates and enqueues a new job
	SubmitJob(ctx context.Context, threadID, message string, user *domain.User) (*domain.Job, error)
	// GetJobStatus retrieves current status of a job
	GetJobStatus(jobID string) (*domain.Job, error)
	// StreamJobEvents streams real-time events for a job
	StreamJobEvents(ctx context.Context, jobID string) (<-chan interface{}, error)
}

// ChatOrchestrationService coordinates the chat processing workflow
type ChatOrchestrationService interface {
	// Start begins processing jobs from the queue
	Start() error
	// Stop gracefully stops job processing
	Stop() error
}

// EventRouter provides access to job event streams
type EventRouter interface {
	// GetJobEventStream returns a channel for streaming job events
	GetJobEventStream(jobID string) (<-chan interface{}, error)
}
