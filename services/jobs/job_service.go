package jobs

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"
	sdk "school-with-ai/sdk/agent-sdk/core"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// jobService implements the services.JobService interface
type jobService struct {
	queue       domain.JobQueue
	eventRouter EventRouter
}

// EventRouter provides access to job event streams
type EventRouter interface {
	GetJobEventStream(jobID string) (<-chan interface{}, error)
}

// NewJobService creates a new job service
func NewJobService(queue domain.JobQueue, router EventRouter) *jobService {
	return &jobService{
		queue:       queue,
		eventRouter: router,
	}
}

// SubmitJob creates and enqueues a new job
func (s *jobService) SubmitJob(ctx context.Context, threadID, message string, user *domain.User) (*domain.Job, error) {
	// Validate input
	if message == "" {
		return nil, fmt.Errorf("message cannot be empty")
	}
	if user == nil {
		return nil, fmt.Errorf("user cannot be nil")
	}

	// Generate thread ID if not provided
	if threadID == "" {
		threadID = fmt.Sprintf("thread_%s_%d", user.ID, time.Now().UnixNano())
	}

	// Generate unique IDs
	jobID := sdk.GenerateID()
	messageID := aguievents.GenerateMessageID()

	// Create job
	job := &domain.Job{
		ID:        jobID,
		ThreadID:  threadID,
		MessageID: messageID,
		Message:   message,
		UserID:    user.ID,
		Status:    domain.JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Enqueue job
	if err := s.queue.Enqueue(job); err != nil {
		return nil, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return job, nil
}

// GetJobStatus retrieves current status of a job
func (s *jobService) GetJobStatus(jobID string) (*domain.Job, error) {
	job, exists := s.queue.GetJob(jobID)
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	return job, nil
}

// StreamJobEvents streams real-time events for a job
func (s *jobService) StreamJobEvents(ctx context.Context, jobID string) (<-chan interface{}, error) {
	// Verify job exists
	_, exists := s.queue.GetJob(jobID)
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Get event stream from router
	if s.eventRouter == nil {
		return nil, fmt.Errorf("event router not configured")
	}

	return s.eventRouter.GetJobEventStream(jobID)
}
