package taskmgmt

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/logger"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/services"
	"school-with-ai/services/taskmanagement"
	"school-with-ai/services/threadmgmt"
	"time"

	"github.com/hibiken/asynq"
)

// TaskManagementHandler encapsulates dependencies for HTTP handlers for task management.
type TaskManagementHandler struct {
	log                 *logger.Logger
	chatClient          *llm.OpenAIClient
	taskManager         *taskmanagement.AsynqTaskManager
	orchestratorService services.AgentOrchestrator
	threadMgmt          *threadmgmt.Manager
	authService         services.AuthService
	threadQueryService  services.ThreadQueryService
	aguiProjection      agui.AguiProjection
	broadcaster         agui.AguiBroadcaster
}

// NewTaskManagementHandler creates a new HTTP handler for task management.
func NewTaskManagementHandler(
	log *logger.Logger,
	chatClient *llm.OpenAIClient,
	taskManager *taskmanagement.AsynqTaskManager,
	orchestratorService services.AgentOrchestrator,
	threadMgmt *threadmgmt.Manager,
	authService services.AuthService,
	threadQueryService services.ThreadQueryService,
	aguiProjection agui.AguiProjection,
	broadcaster agui.AguiBroadcaster,
) *TaskManagementHandler {
	return &TaskManagementHandler{
		log:                 log,
		chatClient:          chatClient,
		taskManager:         taskManager,
		orchestratorService: orchestratorService,
		threadMgmt:          threadMgmt,
		authService:         authService,
		threadQueryService:  threadQueryService,
		aguiProjection:      aguiProjection,
		broadcaster:         broadcaster,
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

	h.log.Info(r.Context(), "[API] Async chat request received",
		"userID", user.ID,
		"contextID", req.ContextID)

	// Submit task via manager (handles ID generation and payload construction)
	result, err := h.taskManager.SubmitChatTask(r.Context(), user.ID, req.Message, req.ContextID)
	if err != nil {
		h.log.Error(r.Context(), "[API] Failed to submit task", "error", err)
		http.Error(w, fmt.Sprintf("Failed to submit task: %v", err), http.StatusInternalServerError)
		return
	}

	h.log.Info(r.Context(), "[API] Task submitted successfully",
		"jobID", result.JobID,
		"taskID", result.TaskInfo.ID,
		"queue", result.TaskInfo.Queue)

	// Return A2A-compliant task response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted for async operations

	// Create A2A Task
	task := NewTask(result.JobID, result.ContextID, TaskStateSubmitted)
	task.Metadata["streamUrl"] = fmt.Sprintf("/v1/tasks/%s:subscribe", result.JobID)

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

	// 1. Get Task Info to find Thread ID
	// We need to know which thread to subscribe to.
	taskInfo, err := h.taskManager.GetTaskInfo("default", jobID)
	if err != nil {
		// Try to see if it exists in other states or complete?
		h.log.Warn(r.Context(), "[API] Failed to get task info", "jobID", jobID, "error", err)
		// If we can't find the task, we can't get the threadID easily unless we query all threads?
		// Client should ideally pass threadID, but API is /tasks/{id}:subscribe.
		// Assuming task is found. If not, maybe it's already done?
		// If done, we don't need to stream.
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// 2. Extract ThreadID from payload
	var payload taskmanagement.ProcessChatPayload
	if err := json.Unmarshal(taskInfo.Payload, &payload); err != nil {
		h.log.Error(r.Context(), "[API] Failed to unmarshal task payload", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	threadID := payload.ThreadID

	h.log.Info(r.Context(), "[API] Starting SSE stream from Projection", "jobID", jobID, "threadID", threadID)

	// 3. Subscribe to Broadcaster (Live Path)
	eventCh, cancel := h.broadcaster.Subscribe(threadID)
	defer cancel()

	// 4. Send Initial Snapshot (Historical Path - optional but recommended)
	// messages := h.threadQueryService.GetThreadMessages(threadID)
	// if len(messages) > 0 {
	// 	if err := WriteSSEEvent(w, aguievents.NewMessagesSnapshotEvent(messages)); err != nil {
	// 		return
	// 	}
	// 	flusher.Flush()
	// }

	// 5. Stream events
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed
				return
			}
			// Convert Domain Event to Agui Event(s)
			aguiEvents := taskmanagement.ToAguiEvents(event)
			for _, ae := range aguiEvents {
				if err := WriteSSEEvent(w, ae); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

// GetJobStatus retrieves the status of a task (A2A compliant).
func (h *TaskManagementHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		http.Error(w, "taskId is required", http.StatusBadRequest)

		return
	}

	// Try to get task info from Asynq
	taskInfo, err := h.taskManager.GetTaskInfo("default", taskID)

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
		task.Status = NewTaskStatus(TaskStateUnknown, nil)
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
	err := h.taskManager.CancelTask(jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel job: %s", err), http.StatusBadRequest)

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

	// Delete thread using thread manager
	err := h.threadMgmt.DeleteThread(r.Context(), threadID)
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
