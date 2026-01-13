package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/llm"
	"school-with-ai/services"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/taskmanagement"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/hibiken/asynq"
)

// TaskManagementHandler encapsulates dependencies for HTTP handlers for task management.
type TaskManagementHandler struct {
	log               *logger.Logger
	chatClient        *llm.OpenAIClient
	taskMgmtService   *taskmanagement.TaskManagementService
	chatWriteService  *chatwrite.ChatWriteService
	authService       services.AuthService
	projectionService services.ProjectionService
}

// NewTaskManagementHandler creates a new HTTP handler for task management.
func NewTaskManagementHandler(
	log *logger.Logger,
	chatClient *llm.OpenAIClient,
	taskMgmtService *taskmanagement.TaskManagementService,
	chatWriteService *chatwrite.ChatWriteService,
	authService services.AuthService,
	projectionService services.ProjectionService,
) *TaskManagementHandler {
	return &TaskManagementHandler{
		log:               log,
		chatClient:        chatClient,
		taskMgmtService:   taskMgmtService,
		chatWriteService:  chatWriteService,
		authService:       authService,
		projectionService: projectionService,
	}
}

// SendMessageRequest represents the async chat request (A2A compatible).
type SendMessageRequest struct {
	ContextID string `json:"contextId,omitempty"` // Optional: existing context (threadID)
	Message   string `json:"message"`             // The message/input for the task
}

// SendMessage handles POST /api/messages - async chat message processing.
func (h *TaskManagementHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
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

	// Generate context ID (thread ID) if not provided
	contextID := req.ContextID
	if contextID == "" {
		contextID = fmt.Sprintf("thread_%s_%d", user.ID, time.Now().UnixNano())
	}

	// Generate IDs for async task
	jobID := core.GenerateID()
	messageID := aguievents.GenerateMessageID()

	// Create payload for async processing
	payload := taskmanagement.ProcessChatPayload{
		JobID:     jobID,
		ThreadID:  contextID,
		MessageID: messageID,
		Message:   req.Message,
		UserID:    user.ID,
	}

	h.log.Info(r.Context(), "[API] Async chat request received",
		"userID", user.ID,
		"email", user.Email,
		"contextID", contextID,
		"taskID", jobID,
		"messageLength", len(req.Message))

	// Enqueue to Asynq for async processing
	taskInfo, err := h.taskMgmtService.EnqueueJob(payload)
	if err != nil {
		h.log.Error(r.Context(), "[API] Failed to enqueue task", "error", err)
		http.Error(w, "Failed to enqueue task", http.StatusInternalServerError)

		return
	}

	h.log.Info(r.Context(), "[API] Task enqueued successfully",
		"jobID", jobID,
		"taskID", taskInfo.ID,
		"queue", taskInfo.Queue,
		"state", taskInfo.State)

	// Return A2A-compliant task response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted for async operations

	// Create A2A Task
	task := NewTask(jobID, contextID, TaskStateSubmitted)
	task.Metadata["streamUrl"] = fmt.Sprintf("/v1/tasks/%s:subscribe", jobID)

	h.log.Info(r.Context(), "[API] Returning task response",
		"taskID", task.ID,
		"contextID", task.ContextID,
		"status", task.Status.State,
		"streamUrl", task.Metadata["streamUrl"])

	if err := json.NewEncoder(w).Encode(task); err != nil {
		h.log.Error(r.Context(), "[API] Failed to encode response", "error", err)

		return
	}
}

// StreamJobEvents streams SSE events for a specific job.
func (h *TaskManagementHandler) StreamJobEvents(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)

		return
	}

	// Override the server's WriteTimeout for this long-lived SSE connection
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		h.log.Error(r.Context(), "[API] Failed to disable write deadline", "error", err)
		// Continue anyway - connection might still work
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
	var eventBuffer *taskmanagement.EventBuffer

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
			eventBuffer, exists = h.taskMgmtService.GetEventBuffer(jobID)
			if exists {
				goto streamEvents
			}
		}
	}

streamEvents:
	// FIRST: Send MESSAGES_SNAPSHOT from projection (ensures client has current state)
	messages := h.projectionService.GetThreadMessages(eventBuffer.ThreadID)
	if len(messages) > 0 {
		messagesSnapshotEvent := aguievents.NewMessagesSnapshotEvent(messages)
		err := writeSSEEvent(w, messagesSnapshotEvent)
		if err != nil {
			h.log.Error(r.Context(), "[API] Failed to write messages snapshot event", "error", err, "jobID", jobID)

			return
		}
		flusher.Flush()
	}

	// THEN: Stream realtime events from buffer
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
				err := h.translateAndWriteEvent(w, flusher, event)
				if err != nil {
					return
				}
				eventIndex++
			}

			// Check if job is complete
			if eventBuffer.IsClosed() && eventIndex >= eventBuffer.Len() {
				// Cleanup: remove event buffer after all events have been sent
				h.taskMgmtService.RemoveEventBuffer(jobID)
				return
			}
		}
	}
}

// translateAndWriteEvent translates domain events to AGUI events and writes them.
func (h *TaskManagementHandler) translateAndWriteEvent(w http.ResponseWriter, flusher http.Flusher, event any) error {
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

	err := writeSSEEvent(w, aguiEvent)
	if err != nil {
		return err
	}
	flusher.Flush()

	return nil
}

// GetJobStatus retrieves the status of a task (A2A compliant).
func (h *TaskManagementHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		http.Error(w, "taskId is required", http.StatusBadRequest)

		return
	}

	// Try to get task info from Asynq
	taskInfo, err := h.taskMgmtService.GetTaskInfo("default", taskID)

	// Create A2A Task response
	task := Task{
		ID:        taskID,
		ContextID: "", // Would need to be stored/retrieved separately
		Artifacts: []Artifact{},
		History:   []*Message{},
		Metadata:  make(map[string]any),
	}

	// Add stream URL to metadata (not part of core A2A spec)
	task.Metadata["streamUrl"] = fmt.Sprintf("/v1/tasks/%s:subscribe", taskID)

	if err != nil {
		// Task not found in queue - check if completed via event buffer
		_, exists := h.taskMgmtService.GetEventBuffer(taskID)
		if exists {
			task.Status = NewTaskStatus(TaskStateCompleted, nil)
		} else {
			task.Status = NewTaskStatus(TaskStateUnknown, nil)
		}
	} else {
		// Map Asynq state to A2A TaskState
		var state TaskState
		switch taskInfo.State {
		case asynq.TaskStatePending:
			state = TaskStateSubmitted
		case asynq.TaskStateActive:
			state = TaskStateWorking
		case asynq.TaskStateScheduled:
			state = TaskStateSubmitted
		case asynq.TaskStateRetry:
			state = TaskStateWorking
		case asynq.TaskStateArchived:
			state = TaskStateFailed
		case asynq.TaskStateCompleted:
			state = TaskStateCompleted
		default:
			state = TaskStateUnknown
		}

		task.Status = NewTaskStatus(state, nil)

		// Add retry information to metadata (implementation detail, not A2A core)
		if taskInfo.Retried > 0 {
			task.Metadata["retried"] = taskInfo.Retried
			task.Metadata["maxRetry"] = taskInfo.MaxRetry
		}
		if taskInfo.LastErr != "" {
			task.Metadata["lastError"] = taskInfo.LastErr
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)

		return
	}
}

// CancelJob cancels a pending or active job.
func (h *TaskManagementHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)

		return
	}

	// Use Asynq Inspector to cancel task
	err := h.taskMgmtService.CancelTask(jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel job: %v", err), http.StatusBadRequest)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]string{
		"jobId":  jobID,
		"status": "cancelled",
	})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)

		return
	}
}

// DeleteThread deletes a thread.
func (h *TaskManagementHandler) DeleteThread(w http.ResponseWriter, r *http.Request) {
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

	h.log.Info(r.Context(), "Deleting thread", "userID", user.ID, "threadID", threadID)

	// Delete thread using chat service
	err := h.chatWriteService.DeleteThread(r.Context(), threadID)
	if err != nil {
		if errors.Is(err, domain.ErrThreadNotFound) {
			http.Error(w, "Thread not found", http.StatusNotFound)

			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	if err != nil {
		h.log.Error(r.Context(), "[API] Failed to encode delete response", "error", err)
	}
}
