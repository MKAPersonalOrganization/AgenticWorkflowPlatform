package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/shared"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/hibiken/asynq"
)

// Task type constants
const (
	TypeProcessChat = "chat:process"
)

// ProcessChatPayload contains the data needed to process a chat message
type ProcessChatPayload struct {
	JobID     string `json:"job_id"`
	ThreadID  string `json:"thread_id"`
	MessageID string `json:"message_id"`
	Message   string `json:"message"`
	UserID    string `json:"user_id"`
}

// NewProcessChatTask creates a new task for processing a chat message
func NewProcessChatTask(payload ProcessChatPayload) (*asynq.Task, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return asynq.NewTask(TypeProcessChat, payloadBytes,
		asynq.MaxRetry(3),             // Retry up to 3 times on failure
		asynq.Timeout(5*time.Minute),  // 5 minute timeout for LLM calls
		asynq.TaskID(payload.JobID),   // Use job ID as task ID for deduplication
		asynq.Retention(24*time.Hour), // Keep completed tasks for 24 hours
	), nil
}

// ChatWorker processes chat messages
type ChatWorker struct {
	log                  core.EventStore
	state                chatwrite.ThreadStore
	whiteboardProjection interface{ Get(threadID string) string }
	chatClient           chatwrite.ChatClient
	chatWriteService     *chatwrite.ChatWriteService
	eventBufferManager   *EventBufferManager
}

// NewChatWorker creates a new chat worker
func NewChatWorker(
	log core.EventStore,
	state chatwrite.ThreadStore,
	whiteboardProjection interface{ Get(threadID string) string },
	chatClient chatwrite.ChatClient,
	chatWriteService *chatwrite.ChatWriteService,
	eventBufferManager *EventBufferManager,
) *ChatWorker {
	return &ChatWorker{
		log:                  log,
		state:                state,
		whiteboardProjection: whiteboardProjection,
		chatClient:           chatClient,
		chatWriteService:     chatWriteService,
		eventBufferManager:   eventBufferManager,
	}
}

// ProcessTask implements asynq.Handler interface
func (w *ChatWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload ProcessChatPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	slog.Info("[WORKER] Picked up job from Redis queue",
		"jobID", payload.JobID,
		"threadID", payload.ThreadID,
		"message", payload.Message)

	// Process the chat message
	if err := w.processChat(ctx, payload); err != nil {
		return err
	}

	slog.Info("[WORKER] Job completed successfully",
		"jobID", payload.JobID,
		"threadID", payload.ThreadID,
		"note", "Events streamed to client via SSE")
	return nil
}

// processChat processes a single chat message (similar to processJob from worker_pool.go)
func (w *ChatWorker) processChat(ctx context.Context, payload ProcessChatPayload) error {
	// Check if context was cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("task cancelled: %w", ctx.Err())
	}

	// Check if thread exists, if not create it
	_, exists := w.state.Get(payload.ThreadID)
	if !exists {
		if err := w.chatWriteService.CreateThread(ctx, payload.ThreadID, "New Chat"); err != nil {
			slog.Error("Failed to create thread", "jobID", payload.JobID, "threadID", payload.ThreadID, "error", err)
			return fmt.Errorf("failed to create thread: %w", err)
		}
	}

	// Add user message
	if err := w.chatWriteService.AddUserMessage(ctx, payload.ThreadID, payload.MessageID, payload.Message); err != nil {
		slog.Error("Failed to add user message", "jobID", payload.JobID, "error", err)
		return fmt.Errorf("failed to add user message: %w", err)
	}

	// Generate unique run ID
	runID := aguievents.GenerateRunID()

	// Create event emitter for SSE streaming
	emitter := w.eventBufferManager.CreateEmitter(payload.JobID)
	defer w.eventBufferManager.CloseEmitter(payload.JobID)

	// Send RUN_STARTED event
	runStartedEvent := aguievents.NewRunStartedEvent(payload.ThreadID, runID)
	if err := emitter.Emit(runStartedEvent); err != nil {
		slog.Error("Failed to emit run started event", "jobID", payload.JobID, "error", err)
	}

	// Auto-generate thread title if this is the first message
	thread, threadExists := w.state.Get(payload.ThreadID)
	if threadExists && thread.Title == "New Chat" && len(thread.Messages) == 1 {
		title := payload.Message
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		if err := w.chatWriteService.UpdateThreadTitle(ctx, payload.ThreadID, title); err != nil {
			slog.Error("Failed to update thread title", "jobID", payload.JobID, "error", err)
		}

		customEvent := aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]string{
				"threadId": payload.ThreadID,
				"title":    title,
			}),
		)
		if err := emitter.Emit(customEvent); err != nil {
			slog.Error("Failed to emit thread title updated event", "jobID", payload.JobID, "error", err)
		}
	}

	// Execute student-facing agent
	w.continueWithAgentDirect(ctx, payload.ThreadID, "student-facing-agent", emitter, runID)

	// Send RUN_FINISHED event
	runFinishedEvent := aguievents.NewRunFinishedEvent(payload.ThreadID, runID)
	if err := emitter.Emit(runFinishedEvent); err != nil {
		slog.Error("Failed to emit run finished event", "jobID", payload.JobID, "error", err)
	}

	return nil
}

// continueWithAgentDirect runs an agent directly
func (w *ChatWorker) continueWithAgentDirect(ctx context.Context, threadID, agentName string, emitter EventEmitter, runID string) {
	// Check if context was cancelled before starting agent
	if ctx.Err() != nil {
		slog.Warn("Agent execution cancelled", "threadID", threadID, "agent", agentName)
		return
	}

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
	agentConfig := factory(notifier, w.whiteboardProjection)

	// Create agent instance
	var handoffReqCtx *core.RequestContext
	agentInstance := chatwrite.New(
		w.chatClient,
		w.log,
		w.state,
		chatwrite.WithConfig(agentConfig),
		chatwrite.WithShouldContinueIteration(shared.CreateShouldContinueIterationFn()),
		chatwrite.WithRequestContextCapture(&handoffReqCtx),
	)

	// Get the thread
	agentThread, exists := w.state.GetAgentThread(threadID)
	if !exists {
		slog.Error("Thread not found", "threadID", threadID)
		return
	}

	// Generate message ID for this run
	messageID := domain.GenerateMessageID()

	// Run the agent
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
	w.emitMessagesSnapshot(emitter, threadID)

	slog.Info("Agent run completed (direct)", "agent", agentName)

	// Check for agent handoff
	if handoffReqCtx != nil {
		if handoff, ok := middleware.GetHandoff(handoffReqCtx); ok {
			slog.Info("Processing agent handoff",
				"threadID", threadID,
				"from", agentName,
				"to", handoff.TargetAgent,
				"question", handoff.Question)

			// Continue with target agent (recursive)
			w.continueWithAgentDirect(ctx, threadID, handoff.TargetAgent, emitter, runID)
		}
	}
}

// emitMessagesSnapshot emits a MESSAGES_SNAPSHOT event
func (w *ChatWorker) emitMessagesSnapshot(emitter EventEmitter, threadID string) {
	sessionMessages := w.state.Messages(threadID)

	// Convert session messages to agui messages
	aguiMessages := make([]aguievents.Message, 0, len(sessionMessages))
	for _, msg := range sessionMessages {
		aguiMsg := ConvertSessionMessageToAGUI(msg)
		aguiMessages = append(aguiMessages, aguiMsg)
	}

	// Emit MESSAGES_SNAPSHOT event
	messagesSnapshotEvent := aguievents.NewMessagesSnapshotEvent(aguiMessages)
	_ = emitter.Emit(messagesSnapshotEvent)

	// Emit tool call agent mappings
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
