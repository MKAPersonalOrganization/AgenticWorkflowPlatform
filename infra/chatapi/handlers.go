package chatapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/auth"
	sdk "school-with-ai/sdk/agent-sdk/core"
	"school-with-ai/services"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ErrJobNotFound is returned when a job is not found
var ErrJobNotFound = errors.New("job not found")

// writeSSEEvent writes an event to the response writer in SSE format
func writeSSEEvent(w http.ResponseWriter, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	if err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	return nil
}

// Handler encapsulates dependencies for HTTP handlers
type Handler struct {
	state       sdk.ThreadStore
	log         sdk.EventStore
	chatClient  sdk.ChatClient
	queue       domain.JobQueue
	workerPool  *WorkerPool
	chatService *services.ChatService
}

// NewHandler creates a new HTTP handler with the given dependencies
func NewHandler(log sdk.EventStore, state sdk.ThreadStore, chatClient sdk.ChatClient, queue domain.JobQueue, workerPool *WorkerPool, chatService *services.ChatService) *Handler {
	return &Handler{
		state:       state,
		log:         log,
		chatClient:  chatClient,
		queue:       queue,
		workerPool:  workerPool,
		chatService: chatService,
	}
}

// SendMessage handles the /send endpoint by enqueuing a job
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get thread ID and message from query parameters
	threadID := r.URL.Query().Get("threadId")
	if threadID == "" {
		// Create user-specific thread ID
		threadID = fmt.Sprintf("thread_%s_%d", user.ID, time.Now().UnixNano())
	}
	message := r.URL.Query().Get("message")
	if message == "" {
		message = "Hello"
	}

	// Create job with user ID
	jobID := sdk.GenerateID()
	messageID := aguievents.GenerateMessageID()

	job := &domain.Job{
		ID:        jobID,
		ThreadID:  threadID,
		MessageID: messageID,
		Message:   message,
		UserID:    user.ID,
		Status:    domain.JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	slog.Info("Creating job for user",
		"userID", user.ID,
		"email", user.Email,
		"threadID", threadID,
		"jobID", jobID)

	// Enqueue job
	if err := h.queue.Enqueue(job); err != nil {
		http.Error(w, fmt.Sprintf("Failed to enqueue job: %v", err), http.StatusInternalServerError)
		return
	}

	// Return job ID to client
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"jobId":    jobID,
		"threadId": threadID,
		"status":   string(domain.JobStatusPending),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// StreamJobEvents streams SSE events for a specific job
func (h *Handler) StreamJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	// Setup SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get the flusher to flush data immediately
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Wait for job to be picked up by a worker
	ctx := r.Context()
	var eventBuffer *EventBuffer

	// Poll for event buffer until worker picks up job (or client disconnects)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected while waiting
			return
		case <-ticker.C:
			var exists bool
			eventBuffer, exists = h.workerPool.GetEventBuffer(jobID)
			if exists {
				goto streamEvents
			}
		}
	}

streamEvents:
	// Stream events from buffer
	eventIndex := 0
	eventTicker := time.NewTicker(50 * time.Millisecond)
	defer eventTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return

		case <-eventTicker.C:
			// Get new events since last index
			newEvents := eventBuffer.GetEvents(eventIndex)

			for _, event := range newEvents {
				// Translate and write event
				if err := h.translateAndWriteEvent(w, flusher, event); err != nil {
					return
				}
				eventIndex++
			}

			// Check if job is complete
			if eventBuffer.IsClosed() && eventIndex >= eventBuffer.Len() {
				return
			}
		}
	}
}

func (h *Handler) translateAndWriteEvent(w http.ResponseWriter, flusher http.Flusher, event any) error {
	var aguiEvent any

	// Translate domain events to agui events
	switch ev := event.(type) {
	case *sdk.TextMessageStartEvent:
		aguiEvent = aguievents.NewTextMessageStartEvent(
			ev.MessageID,
			aguievents.WithRole(ev.Role),
		)

	case *sdk.TextMessageContentEvent:
		aguiEvent = aguievents.NewTextMessageContentEvent(ev.MessageID, ev.Delta)

	case *sdk.TextMessageEndEvent:
		aguiEvent = aguievents.NewTextMessageEndEvent(ev.MessageID)

	case *sdk.ToolCallStartEvent:
		aguiEvent = aguievents.NewToolCallStartEvent(
			ev.ToolCallID,
			ev.ToolCallName,
			aguievents.WithParentMessageID(ev.ParentMessageID),
		)

	case *sdk.ToolCallArgsEvent:
		aguiEvent = aguievents.NewToolCallArgsEvent(ev.ToolCallID, ev.Delta)

	case *sdk.ToolCallEndEvent:
		aguiEvent = aguievents.NewToolCallEndEvent(ev.ToolCallID)

	case *sdk.ToolCallResultEvent:
		aguiEvent = aguievents.NewToolCallResultEvent(
			ev.MessageID,
			ev.ToolCallID,
			ev.Content,
		)

	default:
		// If it's already an agui event, pass through
		aguiEvent = event
	}

	if err := writeSSEEvent(w, aguiEvent); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// GetActiveJobs retrieves all active jobs for a thread
func (h *Handler) GetActiveJobs(w http.ResponseWriter, r *http.Request) {
	threadID := r.URL.Query().Get("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	activeJobs := h.queue.GetActiveJobsByThread(threadID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(activeJobs); err != nil {
		http.Error(w, "Failed to encode jobs", http.StatusInternalServerError)
		return
	}
}

// CancelJob cancels a pending or active job
func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	// Attempt to cancel the job in queue (if pending)
	if err := h.queue.CancelJob(jobID); err != nil {
		if err == ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		// If not pending, might be processing - try worker pool
		if !h.workerPool.CancelJob(jobID) {
			http.Error(w, fmt.Sprintf("Failed to cancel job: %v", err), http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"jobId":  jobID,
		"status": "cancelled",
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetMessages retrieves all messages for a thread
func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	messages := h.state.Messages(threadID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(messages); err != nil {
		http.Error(w, "Failed to encode messages", http.StatusInternalServerError)
		return
	}
}

// GetThreads retrieves all threads
func (h *Handler) GetThreads(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slog.Info("Getting threads for user", "userID", user.ID, "email", user.Email)

	// TODO: Filter threads by user ID
	// For now, return all threads (will be fixed in a future update)
	threads := h.state.All()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(threads); err != nil {
		http.Error(w, "Failed to encode threads", http.StatusInternalServerError)
		return
	}
}

// DeleteThread deletes a thread
func (h *Handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	slog.Info("Deleting thread", "userID", user.ID, "threadID", threadID)

	// TODO: Verify user owns this thread before allowing deletion

	// Delete thread using chat service
	if err := h.chatService.DeleteThread(r.Context(), threadID); err != nil {
		if errors.Is(err, domain.ErrThreadNotFound) {
			http.Error(w, "Thread not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// emitMessagesSnapshot converts session messages to AG-UI messages and emits MESSAGES_SNAPSHOT event
// nolint:unused // May be used for future debugging/monitoring
func (h *Handler) emitMessagesSnapshot(w http.ResponseWriter, flusher http.Flusher, threadID string) error {
	sessionMessages := h.state.Messages(threadID)

	// Convert session.Message to aguievents.Message
	aguiMessages := make([]aguievents.Message, 0, len(sessionMessages))
	for _, msg := range sessionMessages {
		aguiMsg := ConvertSessionMessageToAGUI(msg)
		aguiMessages = append(aguiMessages, aguiMsg)
	}

	// Emit MESSAGES_SNAPSHOT event
	messagesSnapshotEvent := aguievents.NewMessagesSnapshotEvent(aguiMessages)

	if err := writeSSEEvent(w, messagesSnapshotEvent); err != nil {
		return err
	}
	flusher.Flush()

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
				if err := writeSSEEvent(w, agentEvent); err != nil {
					return err
				}
				flusher.Flush()
			}
		}
	}

	return nil
}
