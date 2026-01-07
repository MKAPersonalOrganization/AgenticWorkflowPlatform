package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/eventstore"
	"school-with-ai/infra/projection/llmcontext"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/llm"
	"school-with-ai/services"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/orchestration"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// AsynqHandler encapsulates dependencies for HTTP handlers using Asynq
type AsynqHandler struct {
	state             *llmcontext.InMemoryLLMContextProjection
	log               *eventstore.ProjectionEventStore
	chatClient        *llm.OpenAIClient
	queue             domain.JobQueue
	asynqServer       *orchestration.AsynqServer
	chatWriteService  *chatwrite.ChatWriteService
	authService       services.AuthService
	projectionService services.ProjectionService
}

// NewAsynqHandler creates a new HTTP handler with Asynq dependencies
func NewAsynqHandler(
	log *eventstore.ProjectionEventStore,
	state *llmcontext.InMemoryLLMContextProjection,
	chatClient *llm.OpenAIClient,
	queue domain.JobQueue,
	asynqServer *orchestration.AsynqServer,
	chatWriteService *chatwrite.ChatWriteService,
	authService services.AuthService,
	projectionService services.ProjectionService,
) *AsynqHandler {
	return &AsynqHandler{
		state:             state,
		log:               log,
		chatClient:        chatClient,
		queue:             queue,
		asynqServer:       asynqServer,
		chatWriteService:  chatWriteService,
		authService:       authService,
		projectionService: projectionService,
	}
}

// SendMessage handles the /send endpoint by enqueuing a job with Asynq
func (h *AsynqHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := h.authService.GetUserFromContext(r.Context())
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
	jobID := core.GenerateID()
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

	slog.Info("Creating job for user with Asynq",
		"userID", user.ID,
		"email", user.Email,
		"threadID", threadID,
		"jobID", jobID)

	// Enqueue job to in-memory queue for tracking
	if err := h.queue.Enqueue(job); err != nil {
		http.Error(w, fmt.Sprintf("Failed to enqueue job: %v", err), http.StatusInternalServerError)
		return
	}

	// Enqueue to Asynq for processing
	if err := h.asynqServer.EnqueueJob(job); err != nil {
		http.Error(w, fmt.Sprintf("Failed to enqueue to Asynq: %v", err), http.StatusInternalServerError)
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
func (h *AsynqHandler) StreamJobEvents(w http.ResponseWriter, r *http.Request) {
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

	// Wait for job to be picked up by Asynq worker
	ctx := r.Context()
	var eventBuffer *orchestration.EventBuffer

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
			eventBuffer, exists = h.asynqServer.GetEventBuffer(jobID)
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

// translateAndWriteEvent translates domain events to AGUI events and writes them
func (h *AsynqHandler) translateAndWriteEvent(w http.ResponseWriter, flusher http.Flusher, event any) error {
	var aguiEvent any

	// Translate domain events to agui events
	switch ev := event.(type) {
	case *core.TextMessageStartEvent:
		aguiEvent = aguievents.NewTextMessageStartEvent(
			ev.MessageID,
			aguievents.WithRole(ev.Role),
		)

	case *core.TextMessageContentEvent:
		aguiEvent = aguievents.NewTextMessageContentEvent(ev.MessageID, ev.Delta)

	case *core.TextMessageEndEvent:
		aguiEvent = aguievents.NewTextMessageEndEvent(ev.MessageID)

	case *core.ToolCallStartEvent:
		aguiEvent = aguievents.NewToolCallStartEvent(
			ev.ToolCallID,
			ev.ToolCallName,
			aguievents.WithParentMessageID(ev.ParentMessageID),
		)

	case *core.ToolCallArgsEvent:
		aguiEvent = aguievents.NewToolCallArgsEvent(ev.ToolCallID, ev.Delta)

	case *core.ToolCallEndEvent:
		aguiEvent = aguievents.NewToolCallEndEvent(ev.ToolCallID)

	case *core.ToolCallResultEvent:
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
func (h *AsynqHandler) GetActiveJobs(w http.ResponseWriter, r *http.Request) {
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
func (h *AsynqHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	// Attempt to cancel the job in queue (if pending)
	if err := h.queue.CancelJob(jobID); err != nil {
		if err == domain.ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to cancel job: %v", err), http.StatusBadRequest)
		return
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
func (h *AsynqHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	thread, exists := h.state.Get(threadID)
	if !exists {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(thread.Messages); err != nil {
		http.Error(w, "Failed to encode messages", http.StatusInternalServerError)
		return
	}
}

// GetThreads retrieves all threads
func (h *AsynqHandler) GetThreads(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	slog.Info("Getting threads for user", "userID", user.ID, "email", user.Email)

	threads := h.state.All()

	// Enrich threads with whiteboard content
	response := make([]ThreadResponse, len(threads))
	for i, thread := range threads {
		response[i] = ThreadResponse{
			Thread:            &thread,
			WhiteboardContent: h.projectionService.GetWhiteboardContent(thread.ID),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode threads", http.StatusInternalServerError)
		return
	}
}

// DeleteThread deletes a thread
func (h *AsynqHandler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := h.authService.GetUserFromContext(r.Context())
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

	// Delete thread using chat service
	if err := h.chatWriteService.DeleteThread(r.Context(), threadID); err != nil {
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
