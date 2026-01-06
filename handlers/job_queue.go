package handlers

import (
	"errors"
	"sync"
	"time"

	"school-with-ai/domain"
)

var ErrQueueEmpty = errors.New("queue is empty")

// InMemoryJobQueue implements domain.JobQueue using in-memory storage
type InMemoryJobQueue struct {
	mu      sync.RWMutex
	jobs    map[string]*domain.Job // jobID -> Job
	pending []*domain.Job          // Queue of pending jobs (FIFO)
}

// NewInMemoryJobQueue creates a new in-memory job queue
func NewInMemoryJobQueue() domain.JobQueue {
	return &InMemoryJobQueue{
		jobs:    make(map[string]*domain.Job),
		pending: make([]*domain.Job, 0),
	}
}

// Enqueue adds a job to the queue
func (q *InMemoryJobQueue) Enqueue(job *domain.Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Store job
	q.jobs[job.ID] = job

	// Add to pending queue
	q.pending = append(q.pending, job)

	return nil
}

// Dequeue retrieves the next pending job from the queue
func (q *InMemoryJobQueue) Dequeue() (*domain.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if queue is empty
	if len(q.pending) == 0 {
		return nil, ErrQueueEmpty
	}

	// Get first job (FIFO)
	job := q.pending[0]
	q.pending = q.pending[1:]

	// Update status to processing
	job.Status = domain.JobStatusProcessing
	job.UpdatedAt = time.Now()

	return job, nil
}

// GetJob retrieves a job by ID
func (q *InMemoryJobQueue) GetJob(jobID string) (*domain.Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	job, exists := q.jobs[jobID]
	return job, exists
}

// UpdateJobStatus updates the status of a job
func (q *InMemoryJobQueue) UpdateJobStatus(jobID string, status domain.JobStatus, errorMsg string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	job.Status = status
	job.Error = errorMsg
	job.UpdatedAt = time.Now()

	return nil
}

// GetActiveJobsByThread retrieves all active jobs for a thread
func (q *InMemoryJobQueue) GetActiveJobsByThread(threadID string) []*domain.Job {
	q.mu.RLock()
	defer q.mu.RUnlock()

	activeJobs := make([]*domain.Job, 0)
	for _, job := range q.jobs {
		if job.ThreadID == threadID && (job.Status == domain.JobStatusPending || job.Status == domain.JobStatusProcessing) {
			activeJobs = append(activeJobs, job)
		}
	}

	return activeJobs
}

// GetPendingJobCountByThread returns count of pending jobs for a thread
func (q *InMemoryJobQueue) GetPendingJobCountByThread(threadID string) int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	count := 0
	for _, job := range q.jobs {
		if job.ThreadID == threadID && job.Status == domain.JobStatusPending {
			count++
		}
	}
	return count
}

// CancelJob marks a job as cancelled (if not already processing)
func (q *InMemoryJobQueue) CancelJob(jobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	job, exists := q.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	// Can only cancel pending jobs
	if job.Status != domain.JobStatusPending {
		return errors.New("job is already processing or completed")
	}

	// Remove from pending queue
	for i, pendingJob := range q.pending {
		if pendingJob.ID == jobID {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			break
		}
	}

	// Update status
	job.Status = domain.JobStatusFailed
	job.Error = "cancelled"
	job.UpdatedAt = time.Now()

	return nil
}
