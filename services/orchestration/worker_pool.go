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
	"school-with-ai/services/chatwrite/agents"
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

	// Create UserMessage event to trigger student-facing agent
	userMessageEvent := &agents.DomainEvent{
		ID:        agents.GenerateEventID(),
		Type:      agents.EventTypeUserMessage,
		Timestamp: time.Now(),
		ThreadID:  job.ThreadID,
		Version:   1,
		Data: map[string]interface{}{
			"message":    job.Message,
			"message_id": job.MessageID,
		},
	}

	// Route to student-facing agent via handler
	wp.continueWithAgent(ctx, job.ThreadID, "student-facing-agent", emitter, runID, userMessageEvent)

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

// continueWithAgent runs the specified agent on the thread using event handlers
func (wp *WorkerPool) continueWithAgent(ctx context.Context, threadID, agentName string, emitter EventEmitter, runID string, triggeringEvent *agents.DomainEvent) {
	slog.Info("Continuing with agent", "threadID", threadID, "agent", agentName)

	// Validate that we have a triggering event
	if triggeringEvent == nil {
		slog.Error("No triggering event provided for agent continuation", "agent", agentName)
		return
	}

	// Create UI notifier for this continuation's event stream
	notifier := NewUINotifier(emitter)

	// Get the agent's contract
	contract, ok := eventbus.GlobalContracts.GetContract(agentName)
	if !ok {
		slog.Error("No contract found for agent", "agent", agentName)
		return
	}

	// Create agent config from contract factory
	agentConfig := contract.Factory(notifier)

	// Create agent instance
	agentInstance := chatwrite.New(
		wp.chatClient,
		wp.log,
		wp.state,
		chatwrite.WithConfig(agentConfig),
		chatwrite.WithToolCallHandler(eventbus.CreateToolCallHandler(eventbus.GlobalContracts, wp.state, wp.whiteboardProjection)),
		chatwrite.WithShouldContinueIteration(eventbus.CreateShouldContinueIterationFn(eventbus.GlobalContracts)),
	)

	// Verify the agent has a handler for this event
	if contract.Handlers == nil {
		slog.Error("Agent has no handlers defined", "agent", agentName)
		return
	}

	handler, hasHandler := contract.Handlers[triggeringEvent.Type]
	if !hasHandler {
		slog.Error("Agent has no handler for event type", "agent", agentName, "eventType", triggeringEvent.Type)
		return
	}

	slog.Info("Executing event handler", "agent", agentName, "eventType", triggeringEvent.Type)

	// Create handler context
	handlerCtx := &agents.EventHandlerContext{
		ThreadID: threadID,
		Event:    triggeringEvent,
		Agent:    agentInstance,
		State:    wp.state,
		Notifier: notifier,
	}

	// Track events before handler execution
	eventsBefore, _ := wp.log.ReadStream(ctx, threadID, 0)
	beforeCount := len(eventsBefore)

	// Execute the handler - it owns the complete logic
	if err := handler(ctx, handlerCtx); err != nil {
		slog.Error("Handler execution failed", "agent", agentName, "eventType", triggeringEvent.Type, "error", err)
		errorEvent := aguievents.NewRunErrorEvent(
			fmt.Sprintf("Agent %s handler failed: %v", agentName, err),
			aguievents.WithErrorCode("HANDLER_ERROR"),
		)
		_ = emitter.Emit(errorEvent)
		return
	}

	// Emit updated messages snapshot
	wp.emitMessagesSnapshot(emitter, threadID)

	slog.Info("Handler execution completed", "agent", agentName, "eventType", triggeringEvent.Type)

	// Check for new routing events emitted during handler execution
	eventsAfter, _ := wp.log.ReadStream(ctx, threadID, 0)
	if len(eventsAfter) > beforeCount {
		// Process new events for routing
		newEvents := eventsAfter[beforeCount:]
		wp.processRoutingEvents(ctx, threadID, newEvents, emitter, runID)
	}
}

// processRoutingEvents checks new events for routing and triggers appropriate agents
func (wp *WorkerPool) processRoutingEvents(ctx context.Context, threadID string, newEvents []core.Event, emitter EventEmitter, runID string) {
	for _, event := range newEvents {
		// Check for agent handoff events (NEW: handoff system)
		if handoffEvent, ok := event.(*core.AgentHandoffEvent); ok {
			slog.Info("Processing agent handoff",
				"threadID", threadID,
				"from", handoffEvent.FromAgent,
				"to", handoffEvent.ToAgent,
				"question", handoffEvent.Question)

			// Create a domain event for the target agent
			targetEvent := &agents.DomainEvent{
				ID:        agents.GenerateEventID(),
				Type:      agents.EventTypeAgentHandoff,
				Timestamp: handoffEvent.Timestamp,
				ThreadID:  threadID,
				Version:   1,
				Data: map[string]interface{}{
					"from_agent": handoffEvent.FromAgent,
					"to_agent":   handoffEvent.ToAgent,
					"question":   handoffEvent.Question,
					"context":    handoffEvent.Context,
				},
			}

			// Route to the target agent
			wp.continueWithAgent(ctx, threadID, handoffEvent.ToAgent, emitter, runID, targetEvent)
			continue
		}

		// Cast to DomainEvent to check type
		domainEvent, ok := event.(*agents.DomainEvent)
		if !ok {
			continue
		}

		// Get all agents subscribed to this event type
		subscribers := eventbus.GlobalContracts.EventBus().GetSubscribers(domainEvent.Type)

		if len(subscribers) == 0 {
			continue
		}

		// Get the agent that emitted this event (if available)
		emittedBy, _ := domainEvent.Data["emitted_by"].(string)

		slog.Info("Routing event to subscribers",
			"eventType", domainEvent.Type,
			"emittedBy", emittedBy,
			"subscribers", subscribers,
			"subscriberCount", len(subscribers))

		// Trigger all subscribed agents (except the one that emitted it)
		for _, agentName := range subscribers {
			// Skip if agent is responding to its own emission
			if emittedBy != "" && agentName == emittedBy {
				slog.Debug("Skipping self-handling",
					"agent", agentName,
					"eventType", domainEvent.Type)
				continue
			}

			wp.continueWithAgent(ctx, threadID, agentName, emitter, runID, domainEvent)
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
