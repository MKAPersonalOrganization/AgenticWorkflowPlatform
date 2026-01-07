package orchestration

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	"school-with-ai/sdk/agentsdk/llm"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/shared"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ChatClient defines the interface for LLM providers
type ChatClient interface {
	GetStreamingResponse(
		ctx context.Context,
		messages []llm.ChatMessage,
		options *llm.ChatOptions,
	) iter.Seq2[llm.ChatResponseUpdate, error]
}

// ThreadStore provides read access to thread state
type ThreadStore interface {
	GetAgentThread(threadID string) (core.AgentThread, bool)
	GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error)
	Get(threadID string) (*domain.Thread, bool)
	Messages(threadID string) []domain.Message
	All() []domain.Thread
	Update(event core.Event) error
}

// WorkerPool manages a pool of workers that process jobs from the queue
type WorkerPool struct {
	queue                domain.JobQueue
	log                  core.EventStore
	state                ThreadStore
	whiteboardProjection interface{ Get(threadID string) string } // WhiteboardReader interface
	chatClient           ChatClient
	chatWriteService     *chatwrite.ChatWriteService
	numWorkers           int
	jobChan              chan *domain.Job // Channel for job delivery (replaces polling)
	ctx                  context.Context
	cancel               context.CancelFunc
	wg                   sync.WaitGroup

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
	NumWorkers int
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(queue domain.JobQueue, log core.EventStore, state ThreadStore, whiteboardProjection interface{ Get(threadID string) string }, chatClient ChatClient, chatWriteService *chatwrite.ChatWriteService, config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	if config.NumWorkers <= 0 {
		config.NumWorkers = 5 // Default to 5 workers
	}

	return &WorkerPool{
		queue:                queue,
		log:                  log,
		state:                state,
		whiteboardProjection: whiteboardProjection,
		chatClient:           chatClient,
		chatWriteService:     chatWriteService,
		numWorkers:           config.NumWorkers,
		jobChan:              make(chan *domain.Job, config.NumWorkers*2), // Buffered channel
		ctx:                  ctx,
		cancel:               cancel,
		eventBuffers:         make(map[string]*EventBuffer),
		threadJobs:           make(map[string]string),
		jobCancels:           make(map[string]context.CancelFunc),
		threadRateLimits:     make(map[string]*time.Ticker),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	slog.Info("Starting worker pool", "numWorkers", wp.numWorkers)

	// Start job dispatcher (dequeues from queue, pushes to channel)
	wp.wg.Add(1)
	go wp.dispatcher()

	// Start workers (process from channel)
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// EmitEventForThread emits an AGUI event to all active jobs for the given thread.
// This is called by the AGUI projection to send real-time events to the UI.
func (wp *WorkerPool) EmitEventForThread(threadID string, event any) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	// Find the active job for this thread
	wp.threadMu.RLock()
	jobID, ok := wp.threadJobs[threadID]
	wp.threadMu.RUnlock()

	if !ok {
		// No active job for this thread - event will be in MESSAGES_SNAPSHOT later
		return nil
	}

	// Emit to the job's event buffer
	if buffer, exists := wp.eventBuffers[jobID]; exists {
		buffer.Append(event)
	}

	return nil
}

// GetEventRouter returns an EventEmitter that routes AGUI events from the projection
// to the appropriate job buffers. This enables real-time UI updates from domain events.
func (wp *WorkerPool) GetEventRouter(threadID string) func(event interface{}) error {
	return func(event interface{}) error {
		return wp.EmitEventForThread(threadID, event)
	}
}

// dispatcher continuously dequeues jobs from the queue and sends them to the job channel
func (wp *WorkerPool) dispatcher() {
	defer wp.wg.Done()

	slog.Info("Job dispatcher started")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			slog.Info("Job dispatcher stopping")
			close(wp.jobChan) // Close channel to signal workers
			return

		case <-ticker.C:
			// Try to dequeue a job
			job, err := wp.queue.Dequeue()
			if err != nil {
				if err != domain.ErrQueueEmpty {
					slog.Error("Failed to dequeue job", "error", err)
				}
				continue
			}

			// Send to job channel (non-blocking with timeout)
			select {
			case wp.jobChan <- job:
				slog.Debug("Job dispatched", "jobID", job.ID)
			case <-time.After(5 * time.Second):
				slog.Warn("Job channel full, requeueing", "jobID", job.ID)
				job.Status = domain.JobStatusPending
				if err := wp.queue.Enqueue(job); err != nil {
					slog.Error("Failed to requeue job", "jobID", job.ID, "error", err)
				}
			case <-wp.ctx.Done():
				return
			}
		}
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

// worker processes jobs from the job channel
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	slog.Info("Worker started", "workerID", id)

	for {
		select {
		case <-wp.ctx.Done():
			slog.Info("Worker stopping", "workerID", id)
			return

		case job, ok := <-wp.jobChan:
			if !ok {
				slog.Info("Worker stopping - job channel closed", "workerID", id)
				return
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

	// Check if thread exists, if not create it using chat service
	_, exists := wp.state.Get(job.ThreadID)
	if !exists {
		if err := wp.chatWriteService.CreateThread(ctx, job.ThreadID, "New Chat"); err != nil {
			slog.Error("Failed to create thread", "jobID", job.ID, "threadID", job.ThreadID, "error", err)
			_ = wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, err.Error())
			return
		}
	}

	// Add user message using chat service
	if err := wp.chatWriteService.AddUserMessage(ctx, job.ThreadID, job.MessageID, job.Message); err != nil {
		slog.Error("Failed to add user message", "jobID", job.ID, "error", err)
		_ = wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, err.Error())
		return
	}

	// Generate unique run ID for this execution
	runID := aguievents.GenerateRunID()

	// Create event emitter that sends to SSE channel
	emitter := wp.createChannelEmitter(job.ID)

	// Send RUN_STARTED event
	runStartedEvent := aguievents.NewRunStartedEvent(
		job.ThreadID,
		runID,
	)
	if err := emitter.Emit(runStartedEvent); err != nil {
		slog.Error("Failed to emit run started event", "jobID", job.ID, "error", err)
	}

	// Auto-generate thread title immediately if this is the first message
	thread, threadExists := wp.state.Get(job.ThreadID)
	if threadExists && thread.Title == "New Chat" && len(thread.Messages) == 1 {
		// Take first 50 characters of first user message as title
		title := job.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		// Update thread title using chat service
		if err := wp.chatWriteService.UpdateThreadTitle(ctx, job.ThreadID, title); err != nil {
			slog.Error("Failed to update thread title", "jobID", job.ID, "error", err)
		}

		// Send CUSTOM event to notify frontend of title update immediately
		customEvent := aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]string{
				"threadId": job.ThreadID,
				"title":    title,
			}),
		)
		if err := emitter.Emit(customEvent); err != nil {
			slog.Error("Failed to emit thread title updated event", "jobID", job.ID, "error", err)
		}
	}

	// Execute student-facing agent directly (tool-based routing)
	wp.continueWithAgentDirect(ctx, job.ThreadID, "student-facing-agent", emitter, runID)

	// Send RUN_FINISHED event
	runFinishedEvent := aguievents.NewRunFinishedEvent(
		job.ThreadID,
		runID,
	)
	if err := emitter.Emit(runFinishedEvent); err != nil {
		slog.Error("Failed to emit run finished event", "jobID", job.ID, "error", err)
	}

	// Update job status to completed
	if err := wp.queue.UpdateJobStatus(job.ID, domain.JobStatusCompleted, ""); err != nil {
		slog.Error("Failed to update job status", "jobID", job.ID, "error", err)
	}

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
	aguiMessages := make([]aguievents.Message, 0, len(sessionMessages))
	for _, msg := range sessionMessages {
		aguiMsg := ConvertSessionMessageToAGUI(msg)
		aguiMessages = append(aguiMessages, aguiMsg)
	}

	// Emit MESSAGES_SNAPSHOT event
	messagesSnapshotEvent := aguievents.NewMessagesSnapshotEvent(aguiMessages)

	_ = emitter.Emit(messagesSnapshotEvent)

	// Emit tool call agent mappings (consistent with streaming flow)
	for _, msg := range sessionMessages {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 && msg.AgentName != "" {
			for _, tc := range msg.ToolCalls {
				agentEvent := aguievents.NewCustomEvent(
					"TOOL_CALL_AGENT",
					aguievents.WithValue(map[string]interface{}{
						"toolCallId": tc.ID,
						"agentName":  msg.AgentName,
					}),
				)
				_ = emitter.Emit(agentEvent)
			}
		}
	}
}

// continueWithAgentDirect runs an agent directly without requiring a domain event handler
// This is used for tool-based handoffs where agents don't have event handlers
func (wp *WorkerPool) continueWithAgentDirect(ctx context.Context, threadID, agentName string, emitter EventEmitter, runID string) {
	slog.Info("Continuing with agent (direct)", "threadID", threadID, "agent", agentName)

	// Create UI notifier for this agent's event stream
	notifier := NewUINotifier(emitter)

	// Get the agent's factory
	factory, ok := shared.GlobalAgents.Get(agentName)
	if !ok {
		slog.Error("No factory found for agent", "agent", agentName)
		return
	}

	// Create agent config from factory
	agentConfig := factory(notifier, wp.whiteboardProjection)

	// Create agent instance
	var handoffReqCtx *core.RequestContext // Capture request context to check for handoffs
	agentInstance := chatwrite.New(
		wp.chatClient,
		wp.log,
		wp.state,
		chatwrite.WithConfig(agentConfig),
		chatwrite.WithShouldContinueIteration(shared.CreateShouldContinueIterationFn()),
		chatwrite.WithRequestContextCapture(&handoffReqCtx), // Capture context to check handoff
	)

	// Get the thread
	agentThread, exists := wp.state.GetAgentThread(threadID)
	if !exists {
		slog.Error("Thread not found", "threadID", threadID)
		return
	}

	// Generate message ID for this run
	messageID := domain.GenerateMessageID()

	// Run the agent with the current thread state
	runOpts := core.DefaultRunOptions()
	err := agentInstance.RunWithThread(ctx, threadID, messageID, agentThread, runOpts)
	if err != nil {
		slog.Error("Agent run failed", "agent", agentName, "error", err)
		errorEvent := aguievents.NewRunErrorEvent(
			fmt.Sprintf("Agent %s failed: %v", agentName, err),
			aguievents.WithErrorCode("AGENT_RUN_ERROR"),
		)
		_ = emitter.Emit(errorEvent)
		return
	}

	// Emit updated messages snapshot
	wp.emitMessagesSnapshot(emitter, threadID)

	slog.Info("Agent run completed (direct)", "agent", agentName)

	// Check if agent requested a handoff via transfer_to_agent tool
	if handoffReqCtx != nil {
		if handoff, ok := middleware.GetHandoff(handoffReqCtx); ok {
			slog.Info("Processing agent handoff",
				"threadID", threadID,
				"from", agentName,
				"to", handoff.TargetAgent,
				"question", handoff.Question)

			// Continue with target agent (recursive)
			wp.continueWithAgentDirect(ctx, threadID, handoff.TargetAgent, emitter, runID)
		}
	}
}
