package taskmgmt

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/services"
	"school-with-ai/services/artifact"
	"school-with-ai/services/memory"
	"school-with-ai/services/taskmanagement"
	"school-with-ai/services/threadmgmt"

	"github.com/hibiken/asynq"
)

// TaskManagementHandler encapsulates dependencies for HTTP handlers for task management.
type TaskManagementHandler struct {
	log                *logger.Logger
	taskManager        *taskmanagement.AsynqTaskManager
	agentProvider      agent.AgentProvider
	authService        services.AuthService
	threadQueryService services.ThreadQueryService
	aguiProjection     agui.AguiProjection
	broadcaster        agui.AguiBroadcaster
	threadMgmt         *threadmgmt.Manager
	artifactStore      artifact.Service
	memoryStore        memory.Service
}

// NewTaskManagementHandler creates a new HTTP handler for task management.
func NewTaskManagementHandler(
	log *logger.Logger,
	taskManager *taskmanagement.AsynqTaskManager,
	agentProvider agent.AgentProvider,
	authService services.AuthService,
	threadQueryService services.ThreadQueryService,
	aguiProjection agui.AguiProjection,
	broadcaster agui.AguiBroadcaster,
	threadMgmt *threadmgmt.Manager,
	artifactStore artifact.Service,
	memoryStore memory.Service,
) *TaskManagementHandler {
	return &TaskManagementHandler{
		log:                log,
		taskManager:        taskManager,
		agentProvider:      agentProvider,
		authService:        authService,
		threadQueryService: threadQueryService,
		aguiProjection:     aguiProjection,
		broadcaster:        broadcaster,
		threadMgmt:         threadMgmt,
		artifactStore:      artifactStore,
		memoryStore:        memoryStore,
	}
}

// SendMessageRequest represents the async chat request.
type SendMessageRequest struct {
	ContextID string `json:"contextId,omitempty"` // Optional: existing context (threadID)
	Message   string `json:"message"`             // The message/input for the task
	AgentName string `json:"agentName,omitempty"` // Optional: specific agent to target
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
	result, err := h.taskManager.SubmitChatTask(r.Context(), user.ID, req.Message, req.ContextID, req.AgentName)
	if err != nil {
		h.log.Error(r.Context(), "[API] Failed to submit task", "error", err)
		http.Error(w, fmt.Sprintf("Failed to submit task: %v", err), http.StatusInternalServerError)
		return
	}

	h.log.Info(r.Context(), "[API] Task submitted successfully",
		"jobID", result.JobID,
		"taskID", result.TaskInfo.ID,
		"queue", result.TaskInfo.Queue)

	// Return task response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted for async operations

	// Create Task
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

// GetJobStatus retrieves the status of a task.
func (h *TaskManagementHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		http.Error(w, "taskId is required", http.StatusBadRequest)

		return
	}

	// Try to get task info from Asynq
	taskInfo, err := h.taskManager.GetTaskInfo("default", taskID)

	// Create Task response
	task := Task{
		ID:        taskID,
		ContextID: "", // Would need to be stored/retrieved separately
		Metadata:  make(map[string]any),
	}

	// Add stream URL to metadata
	task.Metadata["streamUrl"] = fmt.Sprintf("/v1/tasks/%s:subscribe", taskID)

	if err != nil {
		task.Status = NewTaskStatus(TaskStateUnknown)
	} else {
		// Map Asynq state to TaskState
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

		task.Status = NewTaskStatus(state)

		// Add retry information to metadata
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

	// Delete thread using thread management manager
	_, err := h.threadMgmt.DeleteThread(r.Context(), threadID)
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

// UpdateThreadTitleRequest represents request body for renaming a thread.
type UpdateThreadTitleRequest struct {
	Title string `json:"title"`
}

// UpdateThreadTitle renames a thread.
// PATCH /api/agui/threads/{threadId}
func (h *TaskManagementHandler) UpdateThreadTitle(w http.ResponseWriter, r *http.Request) {
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

	var req UpdateThreadTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	h.log.Info(r.Context(), "Renaming thread", "userID", user.ID, "threadID", threadID, "newTitle", req.Title)

	event, err := h.threadMgmt.UpdateThreadTitle(r.Context(), threadID, req.Title)
	if err != nil {
		if errors.Is(err, domain.ErrThreadNotFound) {
			http.Error(w, "Thread not found", http.StatusNotFound)
			return
		}
		h.log.Error(r.Context(), "failed to update thread title", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Dispatch the event manually if the broadcaster isn't already handling it via ESDB subscription
	// In this architecture, durable events are usually broadcasted by the projection updater,
	// but for immediate UI feedback on application-level events, we might want to dispatch here too.
	// However, looking at ProcessEvents in agentorchestrator/service.go, it seems durable events
	// are NOT broadcasted by the emitter if they are durable, to avoid double-broadcasting.
	// But ThreadTitleUpdated is an application event, not an agent event.
	// The ProjectionUpdater will eventually broadcast it once it's in ESDB.

	_ = event // Event can be used for logging or specific logic

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "title": req.Title}); err != nil {
		h.log.Error(r.Context(), "[API] Failed to encode rename response", "error", err)
	}
}

// ListAgents returns a list of available agents.
func (h *TaskManagementHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	agents := h.agentProvider.List()

	response := agent.AgentListResponse{
		Agents: make([]agent.AgentMetadata, 0, len(agents)),
	}

	for _, a := range agents {
		response.Agents = append(response.Agents, agent.AgentMetadata{
			ID:          a.ID(),
			Name:        a.Name(),
			Description: a.Description(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error(r.Context(), "failed to encode agents list", "error", err)
		http.Error(w, "Failed to encode agents list", http.StatusInternalServerError)
	}
}

// ListArtifacts returns a list of artifacts for a thread.
// GET /v1/threads/{threadId}/artifacts
func (h *TaskManagementHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
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

	artifacts, err := h.artifactStore.List(r.Context(), user.ID, threadID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list artifacts", "error", err, "userID", user.ID, "threadID", threadID)
		http.Error(w, "Failed to list artifacts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"artifacts": artifacts}); err != nil {
		h.log.Error(r.Context(), "failed to encode artifacts list", "error", err)
	}
}

// DownloadArtifact downloads a specific artifact.
// GET /v1/threads/{threadId}/artifacts/{fileName}
func (h *TaskManagementHandler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	fileName := r.PathValue("fileName")
	if threadID == "" || fileName == "" {
		http.Error(w, "threadId and fileName are required", http.StatusBadRequest)
		return
	}

	content, err := h.artifactStore.Load(r.Context(), user.ID, threadID, fileName)
	if err != nil {
		if errors.Is(err, artifact.ErrNotFound) {
			http.Error(w, "Artifact not found", http.StatusNotFound)
			return
		}
		h.log.Error(r.Context(), "failed to load artifact", "error", err, "userID", user.ID, "threadID", threadID, "fileName", fileName)
		http.Error(w, "Failed to load artifact", http.StatusInternalServerError)
		return
	}
	defer content.Close()

	// Set content type and disposition
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	// In a real app we might want to detect mime type, but S3/MinIO often returns generic if not set.
	// For now, let the browser handle it.

	io.Copy(w, content)
}

// GetMemory retrieves the user's long-term memory.
// GET /v1/memory
func (h *TaskManagementHandler) GetMemory(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	content, err := h.memoryStore.Load(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to load memory", "error", err, "userID", user.ID)
		http.Error(w, "Failed to load memory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"content": content}); err != nil {
		h.log.Error(r.Context(), "failed to encode memory response", "error", err)
	}
}

// AgentTool represents a tool available to an agent.
type AgentTool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
}

// mockTools returns a hardcoded list of agent tools.
func mockTools() []AgentTool {
	return []AgentTool{
		{ID: "calc-add", Name: "Add", Description: "Add two numbers together", Category: "Calculator", Enabled: true},
		{ID: "calc-subtract", Name: "Subtract", Description: "Subtract one number from another", Category: "Calculator", Enabled: true},
		{ID: "calc-multiply", Name: "Multiply", Description: "Multiply two numbers", Category: "Calculator", Enabled: true},
		{ID: "calc-divide", Name: "Divide", Description: "Divide one number by another", Category: "Calculator", Enabled: true},
		{ID: "calc-power", Name: "Power", Description: "Raise a number to a power", Category: "Calculator", Enabled: false},
		{ID: "calc-sqrt", Name: "Square Root", Description: "Calculate the square root of a number", Category: "Calculator", Enabled: false},
		{ID: "viz-plot", Name: "Plot", Description: "Generate a chart or plot from data", Category: "Visualization", Enabled: true},
		{ID: "viz-animate", Name: "Animate", Description: "Create animated visualizations", Category: "Visualization", Enabled: false},
		{ID: "doc-create", Name: "Create Document", Description: "Create a new markdown document", Category: "Documents", Enabled: true},
		{ID: "doc-update", Name: "Update Document", Description: "Update an existing document", Category: "Documents", Enabled: true},
		{ID: "doc-delete", Name: "Delete Document", Description: "Delete a document", Category: "Documents", Enabled: false},
		{ID: "mem-remember", Name: "Remember", Description: "Store information in long-term memory", Category: "Memory", Enabled: true},
		{ID: "mem-forget", Name: "Forget", Description: "Remove information from memory", Category: "Memory", Enabled: false},
		{ID: "mem-list", Name: "List Memories", Description: "List all stored memories", Category: "Memory", Enabled: true},
		{ID: "mem-clear", Name: "Clear Memory", Description: "Clear all stored memories", Category: "Memory", Enabled: false},
	}
}

// ListAgentTools returns a list of tools available to an agent.
// GET /v1/agents/{agentId}/tools
func (h *TaskManagementHandler) ListAgentTools(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"tools": mockTools()}); err != nil {
		h.log.Error(r.Context(), "failed to encode tools list", "error", err)
	}
}

// ToggleAgentToolRequest represents the request body for toggling a tool.
type ToggleAgentToolRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleAgentTool enables or disables a tool for an agent.
// PUT /v1/agents/{agentId}/tools/{toolId}
func (h *TaskManagementHandler) ToggleAgentTool(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolID := r.PathValue("toolId")
	if agentID == "" || toolID == "" {
		http.Error(w, "agentId and toolId are required", http.StatusBadRequest)
		return
	}

	var req ToggleAgentToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":  "success",
		"toolId":  toolID,
		"enabled": req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle tool response", "error", err)
	}
}
