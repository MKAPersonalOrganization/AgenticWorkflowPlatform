package domain

import "time"

// JobStatus represents the current state of a job
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// Job represents a chat processing job
type Job struct {
	ID        string    `json:"id"`
	ThreadID  string    `json:"threadId"`
	MessageID string    `json:"messageId"`
	Message   string    `json:"message"`
	Status    JobStatus `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Internal tracking (not exposed in JSON by default)
	RetryCount int `json:"-"` // Track retry attempts
	WorkerID   int `json:"-"` // Track which worker is processing
}

// JobQueue defines the interface for job queue operations
type JobQueue interface {
	// Enqueue adds a job to the queue
	Enqueue(job *Job) error

	// Dequeue retrieves the next pending job from the queue
	Dequeue() (*Job, error)

	// GetJob retrieves a job by ID
	GetJob(jobID string) (*Job, bool)

	// UpdateJobStatus updates the status of a job
	UpdateJobStatus(jobID string, status JobStatus, errorMsg string) error

	// GetActiveJobsByThread retrieves all active (pending or processing) jobs for a thread
	GetActiveJobsByThread(threadID string) []*Job

	// GetPendingJobCountByThread returns count of pending jobs for a thread
	GetPendingJobCountByThread(threadID string) int

	// CancelJob marks a job as cancelled (if not already processing)
	CancelJob(jobID string) error
}
