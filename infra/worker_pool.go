package infra

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/agui"
	"school-with-ai/services/agent"
)

// WorkerPool manages a pool of workers that process jobs from the queue
type WorkerPool struct {
	queue              domain.JobQueue
	log                agent.EventLog
	state              agent.ThreadState
	provider           agent.Provider
	instructorRegistry *agent.InstructorRegistry
	numWorkers         int
	pollInterval       time.Duration
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup

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
func NewWorkerPool(queue domain.JobQueue, log agent.EventLog, state agent.ThreadState, provider agent.Provider, instructorRegistry *agent.InstructorRegistry, config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	if config.NumWorkers <= 0 {
		config.NumWorkers = 5 // Default to 5 workers
	}
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond // Default poll interval
	}

	return &WorkerPool{
		queue:              queue,
		log:                log,
		state:              state,
		provider:           provider,
		instructorRegistry: instructorRegistry,
		numWorkers:         config.NumWorkers,
		pollInterval:       config.PollInterval,
		ctx:                ctx,
		cancel:             cancel,
		eventBuffers:       make(map[string]*EventBuffer),
		threadJobs:         make(map[string]string),
		jobCancels:         make(map[string]context.CancelFunc),
		threadRateLimits:   make(map[string]*time.Ticker),
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
				_ = wp.queue.Enqueue(job)
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
		_ = wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, "cancelled")
		return
	default:
	}

	// Check if thread exists, if not emit ThreadCreatedEvent
	_, exists := wp.state.Get(job.ThreadID)
	if !exists {
		createEvent := &agent.ThreadCreatedEvent{
			BaseEvent: agent.BaseEvent{
				Type:      "ThreadCreated",
				Timestamp: time.Now(),
				ThreadID:  job.ThreadID,
			},
			Title: "New Chat",
		}
		_ = wp.log.Append(job.ThreadID, createEvent)
		_ = wp.state.Update(createEvent)
	}

	// Emit UserMessageAddedEvent
	userMsgEvent := &agent.UserMessageAddedEvent{
		BaseEvent: agent.BaseEvent{
			Type:      "UserMessageAdded",
			Timestamp: time.Now(),
			ThreadID:  job.ThreadID,
		},
		MessageID: job.MessageID,
		Content:   job.Message,
	}
	_ = wp.log.Append(job.ThreadID, userMsgEvent)
	_ = wp.state.Update(userMsgEvent)

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
	_ = emitter.Emit(runStartedEvent)	// Auto-generate thread title immediately if this is the first message
	thread, threadExists := wp.state.Get(job.ThreadID)
	if threadExists && thread.Title == "New Chat" && len(thread.Messages) == 1 {
		// Take first 50 characters of first user message as title
		title := job.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		// Emit ThreadTitleUpdatedEvent
		titleEvent := &agent.ThreadTitleUpdatedEvent{
			BaseEvent: agent.BaseEvent{
				Type:      "ThreadTitleUpdated",
				Timestamp: time.Now(),
				ThreadID:  job.ThreadID,
			},
			NewTitle: title,
		}
		_ = wp.log.Append(job.ThreadID, titleEvent)
		_ = wp.state.Update(titleEvent)

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
				"title": title,
			},
		}
		_ = emitter.Emit(customEvent)
	}

	// Generate unique message ID for assistant response
	assistantMessageID := agui.GenerateMessageID()

	// Get conversation history from state
	messages := wp.state.Messages(job.ThreadID)

	// Determine which agent to use
	var agentConfig agent.AgentConfig
	thread, threadExists = wp.state.Get(job.ThreadID)

	// CRITICAL: New user messages MUST always go through student-facing agent first
	// This ensures proper routing and prevents instructors from receiving direct questions
	isNewUserMessage := len(messages) == 0 || messages[len(messages)-1].Role == "user"

	if !threadExists || thread.CurrentInstructor == "" || isNewUserMessage {
		// New thread OR new user message - ALWAYS start with student-facing agent (coordinator)
		agentConfig = wp.instructorRegistry.GetStudentFacingAgent()
	} else {
		// Continuation from instructor callback - use current agent
		agentConfig, _ = wp.instructorRegistry.GetByName(thread.CurrentInstructor)
	}

	// Create agent with the determined config
	agentInstance := agent.NewAgent(
		wp.provider,
		wp.log,
		wp.state,
		agent.WithConfig(agentConfig),
	)

	// Create UI notifier
	notifier := NewUINotifier(emitter)

	// Track agent events during the run
	var nextAgent string
	var agentEventOccurred bool

	// Domain event handler: translates domain events to UI events AND tracks agent events
	domainEventHandler := func(event agent.Event) error {
		// First, handle UI translation
		if err := notifier.HandleDomainEvent(event); err != nil {
			return err
		}

		// Track agent call events (but don't act on them yet - wait for agent to finish)
		switch e := event.(type) {
		case *agent.AgentCalledEvent:
			// Current agent called another agent - we'll continue after this run completes
			nextAgent = e.CalledAgent
			agentEventOccurred = true
		}

		return nil
	}

	// Process with agent - it emits domain events, handler translates to UI
	if err := agentInstance.Run(ctx, job.ThreadID, assistantMessageID, messages, domainEventHandler); err != nil {
		// Check if error was due to cancellation
		if ctx.Err() == context.Canceled {
			slog.Info("Job cancelled during processing", "jobID", job.ID)
			_ = wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, "cancelled")

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
			_ = emitter.Emit(errorEvent)
			return
		}
		slog.Error("Failed to process job", "jobID", job.ID, "error", err)

		// Update job status to failed
		_ = wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, err.Error())

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
		_ = emitter.Emit(errorEvent)
		return
	}

	// Emit final MESSAGES_SNAPSHOT with complete conversation
	wp.emitMessagesSnapshot(emitter, job.ThreadID)

	// Check if we need to continue with another agent
	if agentEventOccurred && nextAgent != "" {
		slog.Info("Continuing with called agent", "jobID", job.ID, "agent", nextAgent)
		wp.continueWithAgent(ctx, job.ThreadID, nextAgent, emitter, runID)
	}

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
	_ = emitter.Emit(runFinishedEvent)

	// Update job status to completed
	_ = wp.queue.UpdateJobStatus(job.ID, domain.JobStatusCompleted, "")

	slog.Info("Job completed", "jobID", job.ID, "threadID", job.ThreadID)
}

// createChannelEmitter creates an event emitter that stores events in a buffer
func (wp *WorkerPool) createChannelEmitter(jobID string) EventEmitter {
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
func (wp *WorkerPool) emitMessagesSnapshot(emitter EventEmitter, threadID string) {
	sessionMessages := wp.state.Messages(threadID)

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

// continueWithAgent runs the specified agent on the thread
func (wp *WorkerPool) continueWithAgent(ctx context.Context, threadID, agentName string, emitter EventEmitter, runID string) {
	slog.Info("Continuing with agent", "threadID", threadID, "agent", agentName)

	// Get agent config
	agentConfig, exists := wp.instructorRegistry.GetByName(agentName)
	if !exists {
		slog.Error("Agent not found", "agent", agentName)
		return
	}

	// Create agent instance
	agentInstance := agent.NewAgent(
		wp.provider,
		wp.log,
		wp.state,
		agent.WithConfig(agentConfig),
	)

	// Get updated messages
	messages := wp.state.Messages(threadID)

	// For specialist instructors, add a trigger message to prompt them to act
	// The student-facing-agent doesn't need this since it's just coordinating
	if agentName != "student-facing-agent" {
		// Get whiteboard content
		thread, exists := wp.state.Get(threadID)
		if exists && thread.WhiteboardContent != "" {
			// Add trigger message
			triggerMsg := domain.Message{
				ID:        agui.GenerateMessageID(),
				Role:      "user",
				Timestamp: time.Now(),
				Content: fmt.Sprintf(`The whiteboard has been updated for you to review. Please read it and provide your expert answer.

WHITEBOARD CONTENT:
%s

Follow your state machine workflow:
1. Read and understand the content on the whiteboard
2. Write your contribution to the whiteboard using write_whiteboard with action="append"
3. Decide next action:
   - IF your answer is complete AND no other specialist needed: call_agent(agent_name="student-facing-agent")
   - IF you need another specialist's expertise: call_agent(agent_name=<specialist>)`, thread.WhiteboardContent),
			}
			messages = append(messages, triggerMsg)
		}
	}

	// Generate message ID for agent's response
	messageID := agui.GenerateMessageID()

	// Create UI notifier
	notifier := NewUINotifier(emitter)

	// Track agent call events during the run
	var nextAgent string
	var agentCallOccurred bool

	// Domain event handler for the agent
	agentEventHandler := func(event agent.Event) error {
		// Translate domain events to UI events
		if err := notifier.HandleDomainEvent(event); err != nil {
			return err
		}

		// Track agent call events (but don't act on them yet - wait for agent to finish)
		if callEvent, ok := event.(*agent.AgentCalledEvent); ok {
			nextAgent = callEvent.CalledAgent
			agentCallOccurred = true
		}

		return nil
	}

	// Run the agent
	if err := agentInstance.Run(ctx, threadID, messageID, messages, agentEventHandler); err != nil {
		slog.Error("Agent failed", "agent", agentName, "error", err)

		// Emit error event
		errorTimestamp := time.Now().UnixMilli()
		errorCode := "AGENT_ERROR"
		errorEvent := &agui.RunErrorEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "RUN_ERROR",
				Timestamp: &errorTimestamp,
			},
			Message: fmt.Sprintf("Agent %s failed: %v", agentName, err),
			Code:    &errorCode,
		}
		emitter.Emit(errorEvent)
		return
	}

	// Emit updated messages snapshot
	wp.emitMessagesSnapshot(emitter, threadID)

	// Check if we need to continue with another agent (chained call)
	if agentCallOccurred && nextAgent != "" {
		slog.Info("Chained call to next agent", "from", agentName, "to", nextAgent)
		wp.continueWithAgent(ctx, threadID, nextAgent, emitter, runID)
	}
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
