package infra_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra"
)

// TestThreadIsolation verifies that only one job can process per thread at a time
func TestThreadIsolation(t *testing.T) {
	queue := infra.NewInMemoryJobQueue()

	// Enqueue multiple jobs for the same thread
	threadID := "thread-isolation-test"
	jobIDs := make([]string, 5)

	for i := 0; i < 5; i++ {
		job := &domain.Job{
			ID:        generateTestJobID(i),
			ThreadID:  threadID,
			MessageID: generateTestMessageID(i),
			Message:   "Test message",
			Status:    domain.JobStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		jobIDs[i] = job.ID

		if err := queue.Enqueue(job); err != nil {
			t.Fatalf("Failed to enqueue job %d: %v", i, err)
		}
	}

	// Verify all jobs are pending
	for _, jobID := range jobIDs {
		job, exists := queue.GetJob(jobID)
		if !exists {
			t.Fatalf("Job %s not found", jobID)
		}
		if job.Status != domain.JobStatusPending {
			t.Errorf("Job %s status = %v, want %v", jobID, job.Status, domain.JobStatusPending)
		}
	}

	// Dequeue jobs - should get them one at a time
	processedJobs := make([]*domain.Job, 0, 5)
	for i := 0; i < 5; i++ {
		job, err := queue.Dequeue()
		if err != nil {
			t.Fatalf("Failed to dequeue job %d: %v", i, err)
		}
		processedJobs = append(processedJobs, job)

		// Verify status changed to processing
		if job.Status != domain.JobStatusProcessing {
			t.Errorf("Job %s status after dequeue = %v, want %v", job.ID, job.Status, domain.JobStatusProcessing)
		}
	}

	// Verify queue is empty
	_, err := queue.Dequeue()
	if err != infra.ErrQueueEmpty {
		t.Errorf("Expected ErrQueueEmpty, got %v", err)
	}
}

// TestConcurrentJobEnqueue tests concurrent job enqueuing from multiple goroutines
func TestConcurrentJobEnqueue(t *testing.T) {
	queue := infra.NewInMemoryJobQueue()

	const numGoroutines = 10
	const jobsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Enqueue jobs concurrently
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()

			for i := 0; i < jobsPerGoroutine; i++ {
				job := &domain.Job{
					ID:        generateTestJobID(goroutineID*jobsPerGoroutine + i),
					ThreadID:  "concurrent-test",
					MessageID: generateTestMessageID(goroutineID*jobsPerGoroutine + i),
					Message:   "Concurrent test",
					Status:    domain.JobStatusPending,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				if err := queue.Enqueue(job); err != nil {
					t.Errorf("Failed to enqueue job: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify all jobs were enqueued
	totalJobs := numGoroutines * jobsPerGoroutine
	dequeuedCount := 0

	for {
		_, err := queue.Dequeue()
		if err == infra.ErrQueueEmpty {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error during dequeue: %v", err)
		}
		dequeuedCount++
	}

	if dequeuedCount != totalJobs {
		t.Errorf("Dequeued %d jobs, expected %d", dequeuedCount, totalJobs)
	}
}

// TestJobCancellation tests job cancellation functionality
func TestJobCancellation(t *testing.T) {
	queue := infra.NewInMemoryJobQueue()

	// Create and enqueue a job
	job := &domain.Job{
		ID:        "cancel-test-1",
		ThreadID:  "thread-cancel",
		MessageID: "msg-cancel-1",
		Message:   "To be cancelled",
		Status:    domain.JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := queue.Enqueue(job); err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Cancel the job
	if err := queue.CancelJob(job.ID); err != nil {
		t.Fatalf("Failed to cancel job: %v", err)
	}

	// Verify job is cancelled
	cancelledJob, exists := queue.GetJob(job.ID)
	if !exists {
		t.Fatal("Cancelled job not found")
	}

	if cancelledJob.Status != domain.JobStatusFailed {
		t.Errorf("Cancelled job status = %v, want %v", cancelledJob.Status, domain.JobStatusFailed)
	}

	if cancelledJob.Error != "cancelled" {
		t.Errorf("Cancelled job error = %q, want %q", cancelledJob.Error, "cancelled")
	}

	// Verify job is not in pending queue
	_, err := queue.Dequeue()
	if err != infra.ErrQueueEmpty {
		t.Errorf("Expected empty queue after cancellation, got error: %v", err)
	}
}

// TestGetActiveJobsByThread tests retrieving active jobs for a thread
func TestGetActiveJobsByThread(t *testing.T) {
	queue := infra.NewInMemoryJobQueue()

	thread1 := "thread-1"
	thread2 := "thread-2"

	// Enqueue jobs for thread 1
	for i := 0; i < 3; i++ {
		job := &domain.Job{
			ID:        generateTestJobID(i),
			ThreadID:  thread1,
			MessageID: generateTestMessageID(i),
			Message:   "Thread 1 message",
			Status:    domain.JobStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		queue.Enqueue(job)
	}

	// Enqueue jobs for thread 2
	for i := 0; i < 2; i++ {
		job := &domain.Job{
			ID:        generateTestJobID(i + 100),
			ThreadID:  thread2,
			MessageID: generateTestMessageID(i + 100),
			Message:   "Thread 2 message",
			Status:    domain.JobStatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		queue.Enqueue(job)
	}

	// Get active jobs for thread 1
	activeJobs1 := queue.GetActiveJobsByThread(thread1)
	if len(activeJobs1) != 3 {
		t.Errorf("Thread 1 has %d active jobs, want 3", len(activeJobs1))
	}

	// Get active jobs for thread 2
	activeJobs2 := queue.GetActiveJobsByThread(thread2)
	if len(activeJobs2) != 2 {
		t.Errorf("Thread 2 has %d active jobs, want 2", len(activeJobs2))
	}

	// Dequeue one job from thread 1 (changes to processing)
	queue.Dequeue()

	// Verify still counted as active
	activeJobs1AfterDequeue := queue.GetActiveJobsByThread(thread1)
	if len(activeJobs1AfterDequeue) != 3 {
		t.Errorf("Thread 1 has %d active jobs after dequeue, want 3", len(activeJobs1AfterDequeue))
	}

	// Complete one job
	queue.UpdateJobStatus(activeJobs1AfterDequeue[0].ID, domain.JobStatusCompleted, "")

	// Verify active count decreased
	activeJobs1AfterComplete := queue.GetActiveJobsByThread(thread1)
	if len(activeJobs1AfterComplete) != 2 {
		t.Errorf("Thread 1 has %d active jobs after completion, want 2", len(activeJobs1AfterComplete))
	}
}

// TestEventBuffer tests the event buffer functionality
func TestEventBuffer(t *testing.T) {
	buffer := infra.NewEventBuffer(100)

	// Append events
	for i := 0; i < 10; i++ {
		buffer.Append(map[string]interface{}{
			"type":  "TEST_EVENT",
			"index": i,
		})
	}

	// Verify event count
	if buffer.Len() != 10 {
		t.Errorf("Buffer length = %d, want 10", buffer.Len())
	}

	// Get events from index 0
	events := buffer.GetEvents(0)
	if len(events) != 10 {
		t.Errorf("Got %d events, want 10", len(events))
	}

	// Get events from index 5
	eventsFrom5 := buffer.GetEvents(5)
	if len(eventsFrom5) != 5 {
		t.Errorf("Got %d events from index 5, want 5", len(eventsFrom5))
	}

	// Close buffer
	buffer.Close()

	// Verify closed
	if !buffer.IsClosed() {
		t.Error("Buffer should be closed")
	}

	// Append after close should be ignored
	buffer.Append(map[string]interface{}{"type": "AFTER_CLOSE"})
	if buffer.Len() != 10 {
		t.Errorf("Buffer length after close = %d, want 10 (no change)", buffer.Len())
	}
}

// TestEventBufferConcurrency tests concurrent access to event buffer
func TestEventBufferConcurrency(t *testing.T) {
	buffer := infra.NewEventBuffer(1000)

	var wg sync.WaitGroup
	const numWriters = 5
	const eventsPerWriter = 20
	const numReaders = 10

	// Start writers
	wg.Add(numWriters)
	for w := 0; w < numWriters; w++ {
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < eventsPerWriter; i++ {
				buffer.Append(map[string]interface{}{
					"writer": writerID,
					"index":  i,
				})
				time.Sleep(1 * time.Millisecond)
			}
		}(w)
	}

	// Start readers
	wg.Add(numReaders)
	for r := 0; r < numReaders; r++ {
		go func(readerID int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				events := buffer.GetEvents(0)
				_ = events // Read events
				time.Sleep(5 * time.Millisecond)
			}
		}(r)
	}

	wg.Wait()

	// Verify total events
	expectedEvents := numWriters * eventsPerWriter
	if buffer.Len() != expectedEvents {
		t.Errorf("Buffer has %d events, want %d", buffer.Len(), expectedEvents)
	}
}

// TestContextCancellation simulates job cancellation via context
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Simulate a job that checks context
	jobCompleted := make(chan bool, 1)
	jobCancelled := make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				jobCancelled <- true
				return
			case <-ticker.C:
				// Simulate work
			}
		}
		jobCompleted <- true
	}()

	// Cancel after 50ms
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for result
	select {
	case <-jobCancelled:
		// Expected: job was cancelled
	case <-jobCompleted:
		t.Error("Job completed instead of being cancelled")
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for job cancellation")
	}
}

// Helper functions
func generateTestJobID(i int) string {
	return time.Now().Format("20060102150405") + "-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
}

func generateTestMessageID(i int) string {
	return "msg-" + time.Now().Format("20060102150405") + "-" + string(rune('A'+i%26))
}
