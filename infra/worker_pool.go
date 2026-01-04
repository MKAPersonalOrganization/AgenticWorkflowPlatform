package infra

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
	"school-with-ai/services/sdk"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// AgentFactory is a function that creates an agent configuration with a given notifier
type AgentFactory func(notifier sdk.EventNotifier) sdk.AgentConfig

// AgentRegistry manages agent factories
// Agents register themselves with the registry
type AgentRegistry struct {
	mu        sync.RWMutex
	factories map[string]AgentFactory
	notifier  sdk.EventNotifier
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry(notifier sdk.EventNotifier) *AgentRegistry {
	return &AgentRegistry{
		factories: make(map[string]AgentFactory),
		notifier:  notifier,
	}
}

// Register adds an agent factory to the registry
func (r *AgentRegistry) Register(name string, factory AgentFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// GetByName returns the agent configuration for a specific agent by name
func (r *AgentRegistry) GetByName(name string) (sdk.AgentConfig, bool) {
	r.mu.RLock()
	factory, exists := r.factories[name]
	r.mu.RUnlock()

	if !exists {
		return sdk.AgentConfig{}, false
	}

	return factory(r.notifier), true
}

// SetNotifier updates the notifier for all agents created by this registry
func (r *AgentRegistry) SetNotifier(notifier sdk.EventNotifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifier = notifier
}

// WorkerPool manages a pool of workers that process jobs from the queue
type WorkerPool struct {
	queue         domain.JobQueue
	log           sdk.EventLog
	state         sdk.ThreadState
	chatClient    sdk.ChatClient
	agentRegistry *AgentRegistry
	numWorkers    int
	pollInterval  time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

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
func NewWorkerPool(queue domain.JobQueue, log sdk.EventLog, state sdk.ThreadState, chatClient sdk.ChatClient, agentRegistry *AgentRegistry, config WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	if config.NumWorkers <= 0 {
		config.NumWorkers = 5 // Default to 5 workers
	}
	if config.PollInterval == 0 {
		config.PollInterval = 100 * time.Millisecond // Default poll interval
	}

	return &WorkerPool{
		queue:            queue,
		log:              log,
		state:            state,
		chatClient:       chatClient,
		agentRegistry:    agentRegistry,
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
		createEvent := &sdk.ThreadCreatedEvent{
			BaseEvent: sdk.BaseEvent{
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
	userMsgEvent := &sdk.UserMessageAddedEvent{
		BaseEvent: sdk.BaseEvent{
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

		// Emit ThreadTitleUpdatedEvent
		titleEvent := &sdk.ThreadTitleUpdatedEvent{
			BaseEvent: sdk.BaseEvent{
				Type:      "ThreadTitleUpdated",
				Timestamp: time.Now(),
				ThreadID:  job.ThreadID,
			},
			NewTitle: title,
		}
		_ = wp.log.Append(job.ThreadID, titleEvent)
		_ = wp.state.Update(titleEvent)

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

	// Generate unique message ID for assistant response
	assistantMessageID := aguievents.GenerateMessageID()

	// Get conversation history from state
	messages := wp.state.Messages(job.ThreadID)

	// Determine which agent to use
	thread, threadExists = wp.state.Get(job.ThreadID)

	// CRITICAL: New user messages MUST always go through student-facing agent first
	// This ensures proper routing and prevents instructors from receiving direct questions
	isNewUserMessage := len(messages) == 0 || messages[len(messages)-1].Role == "user"

	// Create UI notifier for this job's event stream
	notifier := NewUINotifier(emitter)

	// Set the notifier in the registry so all agents created will use it
	wp.agentRegistry.SetNotifier(notifier)

	var agentConfig sdk.AgentConfig
	var agentName string

	if !threadExists || thread.CurrentInstructor == "" || isNewUserMessage {
		// New thread OR new user message - ALWAYS start with student-facing agent (coordinator)
		agentName = "student-facing-agent"
	} else {
		// Continuation from instructor callback - use current agent
		agentName = thread.CurrentInstructor
	}

	var found bool
	agentConfig, found = wp.agentRegistry.GetByName(agentName)
	if !found {
		slog.Error("Agent not found in registry", "agent", agentName)
		return
	}

	// Create agent with the determined config
	agentInstance := sdk.NewAgent(
		wp.chatClient,
		wp.log,
		wp.state,
		sdk.WithConfig(agentConfig),
	)

	// Track agent events during the run
	var nextAgent string
	var agentEventOccurred bool

	// Domain event handler: tracks routing events (UI notifications handled by agent's notifier)
	domainEventHandler := func(event sdk.Event) error {
		// Track routing events (but don't act on them yet - wait for agent to finish)
		switch e := event.(type) {
		case *agents.DomainEvent:
			// Handle unified domain events
			switch e.Type {
			case agents.EventTypeMathQuestion:
				// Math question emitted - activate math instructor
				nextAgent = "math-instructor"
				agentEventOccurred = true

			case agents.EventTypePhysicsQuestion:
				// Physics question emitted - activate physics instructor
				nextAgent = "physics-instructor"
				agentEventOccurred = true

			case agents.EventTypeChemistryQuestion:
				// Chemistry question emitted - activate chemistry instructor
				nextAgent = "chemistry-instructor"
				agentEventOccurred = true

			case agents.EventTypeStatisticsQuestion:
				// Statistics question emitted - activate statistics instructor
				nextAgent = "statistics-instructor"
				agentEventOccurred = true

			case agents.EventTypeLinearAlgebraQuestion:
				// Linear algebra question emitted - activate linear algebra professor
				slog.Info("LinearAlgebraQuestion event detected", "nextAgent", "linearalgebra-professor")
				nextAgent = "linearalgebra-professor"
				agentEventOccurred = true

			case agents.EventTypeDiagrammingQuestion:
				// Diagramming question emitted - activate diagramming instructor
				slog.Info("DiagrammingQuestion event detected", "nextAgent", "diagramming-instructor")
				nextAgent = "diagramming-instructor"
				agentEventOccurred = true

			case agents.EventTypeQuestionAnswered:
				// Question answered - activate student-facing agent to review and respond
				nextAgent = "student-facing-agent"
				agentEventOccurred = true
			}
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
			errorEvent := aguievents.NewRunErrorEvent(
				"Job was cancelled",
				aguievents.WithErrorCode("CANCELLED"),
			)
			_ = emitter.Emit(errorEvent)
			return
		}
		slog.Error("Failed to process job", "jobID", job.ID, "error", err)

		// Update job status to failed
		_ = wp.queue.UpdateJobStatus(job.ID, domain.JobStatusFailed, err.Error())

		// Emit RUN_ERROR event
		errorEvent := aguievents.NewRunErrorEvent(
			err.Error(),
			aguievents.WithErrorCode("LLM_ERROR"),
		)
		_ = emitter.Emit(errorEvent)
		return
	}

	// Emit final MESSAGES_SNAPSHOT with complete conversation
	wp.emitMessagesSnapshot(emitter, job.ThreadID)

	// Check if we need to continue with another agent
	// Important: Don't route to the same agent that just ran (agents shouldn't handle their own events)
	if agentEventOccurred && nextAgent != "" && nextAgent != agentName {
		slog.Info("Continuing with called agent", "jobID", job.ID, "currentAgent", agentName, "nextAgent", nextAgent)
		wp.continueWithAgent(ctx, job.ThreadID, nextAgent, emitter, runID)
	} else if agentEventOccurred && nextAgent != "" {
		slog.Warn("Agent tried to route to itself - blocked", "agent", agentName, "nextAgent", nextAgent)
	}

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
		aguiMsg := convertSessionMessageToAGUI(msg)
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

// continueWithAgent runs the specified agent on the thread
func (wp *WorkerPool) continueWithAgent(ctx context.Context, threadID, agentName string, emitter EventEmitter, runID string) {
	slog.Info("Continuing with agent", "threadID", threadID, "agent", agentName)

	// Create UI notifier for this continuation's event stream
	notifier := NewUINotifier(emitter)

	// Set the notifier in the registry so all agents created will use it
	wp.agentRegistry.SetNotifier(notifier)

	// Get agent config
	agentConfig, exists := wp.agentRegistry.GetByName(agentName)
	if !exists {
		slog.Error("Agent not found", "agent", agentName)
		return
	}

	// Create agent instance
	agentInstance := sdk.NewAgent(
		wp.chatClient,
		wp.log,
		wp.state,
		sdk.WithConfig(agentConfig),
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
				ID:        aguievents.GenerateMessageID(),
				Role:      "user",
				Timestamp: time.Now(),
				Content: fmt.Sprintf(`The whiteboard has been updated for you to review. Please read it and provide your expert answer.

WHITEBOARD CONTENT:
%s

Follow your state machine workflow:
1. Read and understand the content on the whiteboard
2. Write your contribution to the whiteboard using write_whiteboard with action="append"
3. Signal completion: emit_question_answered(subject="your_subject", is_complete=true/false)`, thread.WhiteboardContent),
			}
			messages = append(messages, triggerMsg)
		}
	}

	// Generate message ID for agent's response
	messageID := aguievents.GenerateMessageID()

	// Track routing events during the run
	var nextAgent string
	var routingEventOccurred bool

	// Domain event handler for the agent (routing only - UI handled by agent's notifier)
	agentEventHandler := func(event sdk.Event) error {
		// Track routing events (but don't act on them yet - wait for agent to finish)
		switch e := event.(type) {
		case *agents.DomainEvent:
			// Handle unified domain events
			switch e.Type {
			case agents.EventTypeMathQuestion:
				nextAgent = "math-instructor"
				routingEventOccurred = true

			case agents.EventTypePhysicsQuestion:
				nextAgent = "physics-instructor"
				routingEventOccurred = true

			case agents.EventTypeChemistryQuestion:
				nextAgent = "chemistry-instructor"
				routingEventOccurred = true

			case agents.EventTypeStatisticsQuestion:
				nextAgent = "statistics-instructor"
				routingEventOccurred = true

			case agents.EventTypeLinearAlgebraQuestion:
				nextAgent = "linearalgebra-professor"
				routingEventOccurred = true

			case agents.EventTypeDiagrammingQuestion:
				nextAgent = "diagramming-instructor"
				routingEventOccurred = true

			case agents.EventTypeQuestionAnswered:
				nextAgent = "student-facing-agent"
				routingEventOccurred = true
			}
		}

		return nil
	}

	// Run the agent
	if err := agentInstance.Run(ctx, threadID, messageID, messages, agentEventHandler); err != nil {
		slog.Error("Agent failed", "agent", agentName, "error", err)

		// Emit error event
		errorEvent := aguievents.NewRunErrorEvent(
			fmt.Sprintf("Agent %s failed: %v", agentName, err),
			aguievents.WithErrorCode("AGENT_ERROR"),
		)
		_ = emitter.Emit(errorEvent)
		return
	}

	// Emit updated messages snapshot
	wp.emitMessagesSnapshot(emitter, threadID)

	// Check if we need to continue with another agent (chained routing)
	// Important: Don't route to the same agent (agents shouldn't handle their own events)
	if routingEventOccurred && nextAgent != "" && nextAgent != agentName {
		slog.Info("Routing to next agent", "from", agentName, "to", nextAgent)
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

// BufferedEventEmitter implements sdk.EventEmitter using EventBuffer
type BufferedEventEmitter struct {
	jobID  string
	buffer *EventBuffer
}

// Emit appends an event to the buffer
func (e *BufferedEventEmitter) Emit(event any) error {
	e.buffer.Append(event)
	return nil
}
