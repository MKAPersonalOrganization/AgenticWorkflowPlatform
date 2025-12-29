package infra

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/agui"
	"school-with-ai/services/agent"
	"school-with-ai/services/llm"
	"school-with-ai/services/tools"
)

// WorkerPool manages a pool of workers that process jobs from the queue
type WorkerPool struct {
	queue        domain.JobQueue
	store        agent.Repository
	provider     llm.Provider
	numWorkers   int
	pollInterval time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup

	// Event storage for jobs (jobID -> buffered events)
	mu           sync.RWMutex
	eventBuffers map[string]*EventBuffer

	// Thread isolation and concurrency control
	threadMu         sync.RWMutex
	threadJobs       map[string]string             // threadID -> active jobID
	jobCancels       map[string]context.CancelFunc // jobID -> cancel function
	threadRateLimits map[string]*time.Ticker       // threadID -> rate limiter
}

// WorkerPoolConfig configures the worker pool
type WorkerPoolConfig struct {
	NumWorkers   int
	PollInterval time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(queue domain.JobQueue, store agent.Repository, provider llm.Provider, config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	if config.NumWorkers <= 0 {
		config.NumWorkers = 5 // Default to 5 workers
	}
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond // Default poll interval
	}

	return &WorkerPool{
		queue:            queue,
		store:            store,
		provider:         provider,
		numWorkers:       config.NumWorkers,
		pollInterval:     config.PollInterval,
		ctx:              ctx,
		cancel:           cancel,
		eventBuffers:     make(map[string]*EventBuffer),
		threadJobs:       make(map[string]string),
		jobCancels:       make(map[string]context.CancelFunc),
		threadRateLimits: make(map[string]*time.Ticker),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	slog.Info("Starting worker pool", "numWorkers", wp.numWorkers)

	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	slog.Info("Stopping worker pool")
	wp.cancel()
	wp.wg.Wait()

	// Cancel all active jobs
	wp.threadMu.Lock()
	for jobID, cancel := range wp.jobCancels {
		cancel()
		delete(wp.jobCancels, jobID)
	}
	for threadID := range wp.threadJobs {
		delete(wp.threadJobs, threadID)
	}
	for threadID, ticker := range wp.threadRateLimits {
		ticker.Stop()
		delete(wp.threadRateLimits, threadID)
	}
	wp.threadMu.Unlock()

	// Close all event buffers
	wp.mu.Lock()
	for jobID, buffer := range wp.eventBuffers {
		buffer.Close()
		delete(wp.eventBuffers, jobID)
	}
	wp.mu.Unlock()

	slog.Info("Worker pool stopped")
}

// worker is a single worker that polls the queue and processes jobs
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	slog.Info("Worker started", "workerID", id)

	ticker := time.NewTicker(wp.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			slog.Info("Worker stopping", "workerID", id)
			return

		case <-ticker.C:
			// Try to dequeue a job
			job, err := wp.queue.Dequeue()
			if err != nil {
				if err != ErrQueueEmpty {
					slog.Error("Failed to dequeue job", "workerID", id, "error", err)
				}
				continue
			}

			slog.Info("Worker processing job", "workerID", id, "jobID", job.ID, "threadID", job.ThreadID)

			// Check thread isolation - ensure no other job is running for this thread
			wp.threadMu.Lock()
			if existingJobID, exists := wp.threadJobs[job.ThreadID]; exists {
				// Another job is already processing for this thread
				slog.Warn("Thread already has active job, requeueing",
					"workerID", id,
					"jobID", job.ID,
					"threadID", job.ThreadID,
					"existingJobID", existingJobID)

				// Requeue the job and continue
				job.Status = domain.JobStatusPending
				wp.queue.Enqueue(job)
				wp.threadMu.Unlock()
				continue
			}

			// Mark thread as busy with this job
			wp.threadJobs[job.ThreadID] = job.ID

			// Create cancellable context for this job
			jobCtx, jobCancel := context.WithCancel(wp.ctx)
			wp.jobCancels[job.ID] = jobCancel
			wp.threadMu.Unlock()

			// Process the job with cancellable context
			wp.processJob(jobCtx, job)

			// Clean up thread tracking
			wp.threadMu.Lock()
			delete(wp.threadJobs, job.ThreadID)
			delete(wp.jobCancels, job.ID)
			wp.threadMu.Unlock()
		}
	}
}

// processJob processes a single job with context for cancellation
func (wp *WorkerPool) processJob(ctx context.Context, job *domain.Job) {
	// Check if job was cancelled before starting
	select {
	case <-ctx.Done():
		slog.Info("Job cancelled before processing", "jobID", job.ID)
		wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, "cancelled")
		return
	default:
	}

	// Ensure thread exists
	wp.store.GetOrCreateThread(job.ThreadID)

	// Store user message
	wp.store.AddMessage(job.ThreadID, domain.Message{
		ID:        job.MessageID,
		Role:      "user",
		Content:   job.Message,
		Timestamp: time.Now(),
	})

	// Generate unique run ID for this execution
	runID := agui.GenerateRunID()

	// Create event emitter that sends to SSE channel
	emitter := wp.createChannelEmitter(job.ID)

	// Send RUN_STARTED event
	runStartedTimestamp := time.Now().UnixMilli()
	runStartedEvent := &agui.RunStartedEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "RUN_STARTED",
			Timestamp: &runStartedTimestamp,
		},
		ThreadID: job.ThreadID,
		RunID:    runID,
	}
	emitter.Emit(runStartedEvent)

	// Auto-generate thread title immediately if this is the first message
	thread, exists := wp.store.GetThread(job.ThreadID)
	if exists && thread.Title == "New Chat" && len(thread.Messages) == 1 {
		// Take first 50 characters of first user message as title
		title := job.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		wp.store.UpdateThreadTitle(job.ThreadID, title)

		// Send CUSTOM event to notify frontend of title update immediately
		customTimestamp := time.Now().UnixMilli()
		customEvent := &agui.CustomEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "CUSTOM",
				Timestamp: &customTimestamp,
			},
			Name: "THREAD_TITLE_UPDATED",
			Value: map[string]string{
				"threadId": job.ThreadID,
				"title":    title,
			},
		}
		emitter.Emit(customEvent)
	}

	// Generate unique message ID for assistant response
	assistantMessageID := agui.GenerateMessageID()

	// Get conversation history
	messages := wp.store.GetMessages(job.ThreadID)

	// Get available tools
	availableTools := tools.GetAllTools()

	// Create agent with tools
	messageHandler := agent.NewAgent(
		wp.provider,
		wp.store,
		emitter,
		agent.WithTools(availableTools),
	)

	// Process user message and stream response with cancellable context
	if err := messageHandler.Run(ctx, job.ThreadID, assistantMessageID, messages); err != nil {
		// Check if error was due to cancellation
		if ctx.Err() == context.Canceled {
			slog.Info("Job cancelled during processing", "jobID", job.ID)
			wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, "cancelled")

			// Save partial response before emitting error
			wp.emitMessagesSnapshot(emitter, job.ThreadID)

			// Emit RUN_ERROR event for cancellation
			errorTimestamp := time.Now().UnixMilli()
			errorCode := "CANCELLED"
			errorEvent := &agui.RunErrorEvent{
				BaseEvent: agui.BaseEvent{
					Type:      "RUN_ERROR",
					Timestamp: &errorTimestamp,
				},
				Message: "Job was cancelled",
				Code:    &errorCode,
			}
			emitter.Emit(errorEvent)
			return
		}
		slog.Error("Failed to process job", "jobID", job.ID, "error", err)

		// Update job status to failed
		wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, err.Error())

		// Emit RUN_ERROR event
		errorTimestamp := time.Now().UnixMilli()
		errorCode := "LLM_ERROR"
		errorEvent := &agui.RunErrorEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "RUN_ERROR",
				Timestamp: &errorTimestamp,
			},
			Message: err.Error(),
			Code:    &errorCode,
		}
		emitter.Emit(errorEvent)
		return
	}

	// Emit final MESSAGES_SNAPSHOT with complete conversation
	wp.emitMessagesSnapshot(emitter, job.ThreadID)

	// Send RUN_FINISHED event
	runFinishedTimestamp := time.Now().UnixMilli()
	runFinishedEvent := &agui.RunFinishedEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "RUN_FINISHED",
			Timestamp: &runFinishedTimestamp,
		},
		ThreadID: job.ThreadID,
		RunID:    runID,
	}
	emitter.Emit(runFinishedEvent)

	// Update job status to completed
	wp.queue.UpdateJobStatus(job.ID, domain.JobStatusCompleted, "")

	slog.Info("Job completed", "jobID", job.ID, "threadID", job.ThreadID)
}

// createChannelEmitter creates an event emitter that stores events in a buffer
func (wp *WorkerPool) createChannelEmitter(jobID string) agent.EventEmitter {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	// Create a buffered event storage
	buffer := NewEventBuffer(1000) // Store up to 1000 events
	wp.eventBuffers[jobID] = buffer

	return &BufferedEventEmitter{
		jobID:  jobID,
		buffer: buffer,
	}
}

// GetEventBuffer gets the event buffer for a job
func (wp *WorkerPool) GetEventBuffer(jobID string) (*EventBuffer, bool) {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	buffer, exists := wp.eventBuffers[jobID]
	return buffer, exists
}

// RemoveEventBuffer removes the event buffer for a job
func (wp *WorkerPool) RemoveEventBuffer(jobID string) {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if buffer, exists := wp.eventBuffers[jobID]; exists {
		buffer.Close()
		delete(wp.eventBuffers, jobID)
	}
}

// CancelJob attempts to cancel a job that is currently processing
func (wp *WorkerPool) CancelJob(jobID string) bool {
	wp.threadMu.Lock()
	defer wp.threadMu.Unlock()

	if cancel, exists := wp.jobCancels[jobID]; exists {
		slog.Info("Cancelling job", "jobID", jobID)
		cancel()
		return true
	}

	return false
}

// emitMessagesSnapshot emits a MESSAGES_SNAPSHOT event
func (wp *WorkerPool) emitMessagesSnapshot(emitter agent.EventEmitter, threadID string) {
	sessionMessages := wp.store.GetMessages(threadID)

	// Convert session messages to agui messages
	aguiMessages := make([]agui.Message, 0, len(sessionMessages))
	for _, msg := range sessionMessages {
		aguiMsg := convertSessionMessageToAGUI(msg)
		aguiMessages = append(aguiMessages, aguiMsg)
	}

	// Emit MESSAGES_SNAPSHOT event
	timestamp := time.Now().UnixMilli()
	messagesSnapshotEvent := &agui.MessagesSnapshotEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "MESSAGES_SNAPSHOT",
			Timestamp: &timestamp,
		},
		Messages: aguiMessages,
	}

	emitter.Emit(messagesSnapshotEvent)
}

// EventBuffer stores events for a job and allows multiple consumers
type EventBuffer struct {
	mu      sync.RWMutex
	events  []any
	closed  bool
	maxSize int
}

// NewEventBuffer creates a new event buffer
func NewEventBuffer(maxSize int) *EventBuffer {
	return &EventBuffer{
		events:  make([]any, 0, maxSize),
		maxSize: maxSize,
	}
}

// Append adds an event to the buffer
func (eb *EventBuffer) Append(event any) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}

	eb.events = append(eb.events, event)

	// Trim if exceeds max size (keep most recent events)
	if len(eb.events) > eb.maxSize {
		eb.events = eb.events[len(eb.events)-eb.maxSize:]
	}
}

// GetEvents returns all events since the given index
func (eb *EventBuffer) GetEvents(fromIndex int) []any {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if fromIndex >= len(eb.events) {
		return nil
	}

	return eb.events[fromIndex:]
}

// Len returns the number of events
func (eb *EventBuffer) Len() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.events)
}

// Close marks the buffer as closed
func (eb *EventBuffer) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.closed = true
}

// IsClosed returns whether the buffer is closed
func (eb *EventBuffer) IsClosed() bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.closed
}

// BufferedEventEmitter implements agent.EventEmitter using EventBuffer
type BufferedEventEmitter struct {
	jobID  string
	buffer *EventBuffer
}

// Emit appends an event to the buffer
func (e *BufferedEventEmitter) Emit(event any) error {
	e.buffer.Append(event)
	return nil
}
