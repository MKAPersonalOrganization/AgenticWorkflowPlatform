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
	"github.com/hibiken/asynq"
)

// AsynqHandler encapsulates dependencies for HTTP handlers using Asynq
type AsynqHandler struct {
	state             *llmcontext.InMemoryLLMContextProjection
	log               *eventstore.ProjectionEventStore
	chatClient        *llm.OpenAIClient
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
	asynqServer *orchestration.AsynqServer,
	chatWriteService *chatwrite.ChatWriteService,
	authService services.AuthService,
	projectionService services.ProjectionService,
) *AsynqHandler {
	return &AsynqHandler{
		state:             state,
		log:               log,
		chatClient:        chatClient,
		asynqServer:       asynqServer,
		chatWriteService:  chatWriteService,
		authService:       authService,
		projectionService: projectionService,
	}
}

// SendMessageRequest represents the async chat request
type SendMessageRequest struct {
	ThreadID string `json:"threadId,omitempty"`
	Message  string `json:"message"`
}

// SendMessageResponse represents the async job response
type SendMessageResponse struct {
	JobID     string `json:"jobId"`
	ThreadID  string `json:"threadId"`
	Status    string `json:"status"`
	Queue     string `json:"queue"`
	StreamURL string `json:"streamUrl"`
	CreatedAt string `json:"createdAt"`
}

// SendMessage handles POST /api/messages - async chat message processing
func (h *AsynqHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse JSON request body
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Generate thread ID if not provided
	threadID := req.ThreadID
	if threadID == "" {
		threadID = fmt.Sprintf("thread_%s_%d", user.ID, time.Now().UnixNano())
	}

	// Generate IDs for async task
	jobID := core.GenerateID()
	messageID := aguievents.GenerateMessageID()

	// Create payload for async processing
	payload := orchestration.ProcessChatPayload{
		JobID:     jobID,
		ThreadID:  threadID,
		MessageID: messageID,
		Message:   req.Message,
		UserID:    user.ID,
	}

	slog.Info("[API] Async chat request received",
		"userID", user.ID,
		"email", user.Email,
		"threadID", threadID,
		"jobID", jobID,
		"messageLength", len(req.Message))

	// Enqueue to Asynq for async processing
	taskInfo, err := h.asynqServer.EnqueueJob(payload)
	if err != nil {
		slog.Error("[API] Failed to enqueue task", "error", err)
		http.Error(w, "Failed to enqueue task", http.StatusInternalServerError)
		return
	}

	slog.Info("[API] Task enqueued successfully",
		"jobID", jobID,
		"taskID", taskInfo.ID,
		"queue", taskInfo.Queue,
		"state", taskInfo.State)

	// Return async job response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted for async operations

	response := SendMessageResponse{
		JobID:     jobID,
		ThreadID:  threadID,
		Status:    "queued",
		Queue:     taskInfo.Queue,
		StreamURL: fmt.Sprintf("/api/jobs/%s/stream", jobID),
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("[API] Failed to encode response", "error", err)
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

// JobStatusResponse represents async job status
type JobStatusResponse struct {
	JobID       string `json:"jobId"`
	Status      string `json:"status"`
	Queue       string `json:"queue,omitempty"`
	Retried     int    `json:"retried,omitempty"`
	MaxRetry    int    `json:"maxRetry,omitempty"`
	LastError   string `json:"lastError,omitempty"`
	StreamURL   string `json:"streamUrl"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// GetJobStatus retrieves the status of an async task
func (h *AsynqHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	// Try to get task info from Asynq
	taskInfo, err := h.asynqServer.GetTaskInfo("default", jobID)

	response := JobStatusResponse{
		JobID:     jobID,
		StreamURL: fmt.Sprintf("/api/jobs/%s/stream", jobID),
	}

	if err != nil {
		// Task not found in queue - might be completed or never existed
		// Check if event buffer exists (means task was processed)
		_, exists := h.asynqServer.GetEventBuffer(jobID)
		if exists {
			response.Status = "completed"
		} else {
			response.Status = "not_found"
		}
	} else {
		// Map Asynq state to our status
		switch taskInfo.State {
		case asynq.TaskStatePending:
			response.Status = "pending"
		case asynq.TaskStateActive:
			response.Status = "processing"
		case asynq.TaskStateScheduled:
			response.Status = "scheduled"
		case asynq.TaskStateRetry:
			response.Status = "retrying"
		case asynq.TaskStateArchived:
			response.Status = "failed"
		case asynq.TaskStateCompleted:
			response.Status = "completed"
			if !taskInfo.CompletedAt.IsZero() {
				response.CompletedAt = taskInfo.CompletedAt.Format(time.RFC3339)
			}
		default:
			response.Status = "unknown"
		}

		response.Queue = taskInfo.Queue
		response.Retried = taskInfo.Retried
		response.MaxRetry = taskInfo.MaxRetry
		if taskInfo.LastErr != "" {
			response.LastError = taskInfo.LastErr
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
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

	// Use Asynq Inspector to cancel task
	if err := h.asynqServer.CancelTask(jobID); err != nil {
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
