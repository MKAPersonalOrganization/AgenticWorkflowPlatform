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
	"school-with-ai/sdk/agentsdk/llm"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"

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
	pollInterval         time.Duration
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
	NumWorkers   int
	PollInterval time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(queue domain.JobQueue, log core.EventStore, state ThreadStore, whiteboardProjection interface{ Get(threadID string) string }, chatClient ChatClient, chatWriteService *chatwrite.ChatWriteService, config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	if config.NumWorkers <= 0 {
		config.NumWorkers = 5 // Default to 5 workers
	}
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond // Default poll interval
	}

	return &WorkerPool{
		queue:                queue,
		log:                  log,
		state:                state,
		whiteboardProjection: whiteboardProjection,
		chatClient:           chatClient,
		chatWriteService:     chatWriteService,
		numWorkers:           config.NumWorkers,
		pollInterval:         config.PollInterval,
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
				if err != domain.ErrQueueEmpty {
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

	// Check if thread exists, if not create it using chat service
	_, exists := wp.state.Get(job.ThreadID)
	if !exists {
		_ = wp.chatWriteService.CreateThread(ctx, job.ThreadID, "New Chat")
	}

	// Add user message using chat service
	_ = wp.chatWriteService.AddUserMessage(ctx, job.ThreadID, job.MessageID, job.Message)

	// Generate unique run ID for this execution
	runID := aguievents.GenerateRunID()

	// Create event emitter that sends to SSE channel
	emitter := wp.createChannelEmitter(job.ID)

	// Send RUN_STARTED event
	runStartedEvent := aguievents.NewRunStartedEvent(
		job.ThreadID,
		runID,
	)
	_ = emitter.Emit(runStartedEvent) // Auto-generate thread title immediately if this is the first message
	thread, threadExists := wp.state.Get(job.ThreadID)
	if threadExists && thread.Title == "New Chat" && len(thread.Messages) == 1 {
		// Take first 50 characters of first user message as title
		title := job.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		// Update thread title using chat service
		_ = wp.chatWriteService.UpdateThreadTitle(ctx, job.ThreadID, title)

		// Send CUSTOM event to notify frontend of title update immediately
		customEvent := aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]string{
				"threadId": job.ThreadID,
				"title":    title,
			}),
		)
		_ = emitter.Emit(customEvent)
	}

	// Execute student-facing agent directly (tool-based routing)
	wp.continueWithAgentDirect(ctx, job.ThreadID, "student-facing-agent", emitter, runID)

	// Send RUN_FINISHED event
	runFinishedEvent := aguievents.NewRunFinishedEvent(
		job.ThreadID,
		runID,
	)
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

	// Get the agent's contract
	contract, ok := eventbus.GlobalContracts.GetContract(agentName)
	if !ok {
		slog.Error("No contract found for agent", "agent", agentName)
		return
	}

	// Create agent config from contract factory
	agentConfig := contract.Factory(notifier, wp.whiteboardProjection)

	// Create agent instance
	agentInstance := chatwrite.New(
		wp.chatClient,
		wp.log,
		wp.state,
		chatwrite.WithConfig(agentConfig),
		chatwrite.WithToolCallHandler(eventbus.CreateToolCallHandler(eventbus.GlobalContracts, wp.state, wp.whiteboardProjection)),
		chatwrite.WithShouldContinueIteration(eventbus.CreateShouldContinueIterationFn(eventbus.GlobalContracts)),
	)

	// Get the thread
	agentThread, exists := wp.state.GetAgentThread(threadID)
	if !exists {
		slog.Error("Thread not found", "threadID", threadID)
		return
	}

	// Track events before agent execution
	eventsBefore, _ := wp.log.ReadStream(ctx, threadID, 0)
	beforeCount := len(eventsBefore)

	// Generate message ID for this run
	messageID := domain.GenerateMessageID()

	// Run the agent with the current thread state
	runOpts := core.DefaultRunOptions()
	err := agentInstance.RunWithThread(ctx, threadID, messageID, agentThread, func(event core.Event) error {
		return nil
	}, runOpts)
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

	// Check for new handoff events emitted during agent execution
	eventsAfter, _ := wp.log.ReadStream(ctx, threadID, 0)
	if len(eventsAfter) > beforeCount {
		// Process new handoff events (e.g., nested handoffs)
		newEvents := eventsAfter[beforeCount:]
		wp.processHandoffEvents(ctx, threadID, newEvents, emitter, runID)
	}
}

// processHandoffEvents checks for agent handoff events and continues execution
func (wp *WorkerPool) processHandoffEvents(ctx context.Context, threadID string, newEvents []core.Event, emitter EventEmitter, runID string) {
	for _, event := range newEvents {
		// Check for agent handoff events (tool-based handoff system)
		if handoffEvent, ok := event.(*core.AgentHandoffEvent); ok {
			slog.Info("Processing tool-based agent handoff",
				"threadID", threadID,
				"from", handoffEvent.FromAgent,
				"to", handoffEvent.ToAgent,
				"question", handoffEvent.Question)

			// Continue directly with the target agent
			wp.continueWithAgentDirect(ctx, threadID, handoffEvent.ToAgent, emitter, runID)
		}
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

// BufferedEventEmitter implements core.EventEmitter using EventBuffer
type BufferedEventEmitter struct {
	jobID  string
	buffer *EventBuffer
}

// Emit appends an event to the buffer
func (e *BufferedEventEmitter) Emit(event any) error {
	e.buffer.Append(event)
	return nil
}
