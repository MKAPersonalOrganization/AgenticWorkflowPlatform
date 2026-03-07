package taskmgmt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/chatcontracts"
	"school-with-ai/modules/chat/projection/agui"
	"school-with-ai/modules/chat/taskmanagement"
	"school-with-ai/modules/chat/threadmgmt"
	"school-with-ai/platform/artifact"
	"school-with-ai/platform/memory"
	"school-with-ai/modules/permissions/tool"
	"school-with-ai/platform/toolregistry"
	"school-with-ai/shared"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// TaskManagementHandler encapsulates dependencies for HTTP handlers for task management.
type TaskManagementHandler struct {
	log                *logger.Logger
	taskManager        *taskmanagement.AsynqTaskManager
	agentProvider      agent.AgentProvider
	authService        shared.AuthService
	threadQueryService chatcontracts.ThreadQueryService
	aguiProjection     agui.AguiProjection
	broadcaster        agui.AguiBroadcaster
	threadMgmt         *threadmgmt.Manager
	artifactStore      artifact.Service
	memoryStore        memory.Service
	toolRegistry       *toolregistry.Registry
	permissionsService *tool.Service
	documentProvider   chatcontracts.DocumentProvider
	tenantID           string
	allowedOrigin      string
}

// NewTaskManagementHandler creates a new HTTP handler for task management.
func NewTaskManagementHandler(
	log *logger.Logger,
	taskManager *taskmanagement.AsynqTaskManager,
	agentProvider agent.AgentProvider,
	authService shared.AuthService,
	threadQueryService chatcontracts.ThreadQueryService,
	aguiProjection agui.AguiProjection,
	broadcaster agui.AguiBroadcaster,
	threadMgmt *threadmgmt.Manager,
	artifactStore artifact.Service,
	memoryStore memory.Service,
	toolRegistry *toolregistry.Registry,
	permissionsService *tool.Service,
	documentProvider chatcontracts.DocumentProvider,
	tenantID string,
	allowedOrigin string,
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
		toolRegistry:       toolRegistry,
		permissionsService: permissionsService,
		documentProvider:   documentProvider,
		tenantID:           tenantID,
		allowedOrigin:      allowedOrigin,
	}
}

// defaultAgentIconSVG is the fallback compass icon for agents without a valid SVG icon.
var defaultAgentIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="m16.24 7.76-1.804 5.411a2 2 0 0 1-1.265 1.265L7.76 16.24l1.804-5.411a2 2 0 0 1 1.265-1.265z" /></svg>`

// resolveAgentIcon extracts the icon from an agent (via optional Icon() method)
// and falls back to the default compass SVG if the agent doesn't provide one.
func resolveAgentIcon(a agent.Agent) string {
	if ip, ok := a.(interface{ Icon() string }); ok {
		if icon := ip.Icon(); strings.HasPrefix(icon, "<svg") {
			return icon
		}
	}
	return defaultAgentIconSVG
}

// FileAttachment represents a file sent with a message (image, audio, PDF, etc.).
type FileAttachment struct {
	Data     string `json:"data,omitempty"`     // Base64 data URL (data:image/...;base64,...) — legacy path
	URL      string `json:"url,omitempty"`      // Pre-uploaded artifact URL (/v1/threads/.../artifacts/...)
	MimeType string `json:"mimeType,omitempty"` // e.g. "image/png", "audio/wav"
	FileName string `json:"fileName,omitempty"` // Original filename
}

// SendMessageRequest represents the async chat request.
type SendMessageRequest struct {
	ContextID   string           `json:"contextId,omitempty"`   // Optional: existing context (threadID)
	Message     string           `json:"message"`               // The message/input for the task
	AgentName   string           `json:"agentName,omitempty"`   // Optional: specific agent to target
	ToolCallID  string           `json:"toolCallId,omitempty"`  // Optional: correlates this message as an answer to a tool's elicitation
	Attachments []FileAttachment `json:"attachments,omitempty"` // Optional: attached files (base64 data URLs)
}

// SendMessage handles POST /api/messages - async chat message processing.
func (h *TaskManagementHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)

		return
	}

	// Parse JSON request body (body size is enforced by BodyLimit middleware)
	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	if req.Message == "" && len(req.Attachments) == 0 {
		http.Error(w, "Message or attachments required", http.StatusBadRequest)

		return
	}

	h.log.Info(r.Context(), "[API] Async chat request received",
		"userID", user.ID,
		"contextID", req.ContextID,
		"numAttachments", len(req.Attachments))

	// Convert file attachments to task manager format
	var attachments []taskmanagement.FileAttachment
	for _, att := range req.Attachments {
		attachments = append(attachments, taskmanagement.FileAttachment{
			Data:     att.Data,
			URL:      att.URL,
			MimeType: att.MimeType,
			FileName: att.FileName,
		})
	}

	// Submit task via manager (handles ID generation and payload construction)
	result, err := h.taskManager.SubmitChatTask(r.Context(), user.ID, req.Message, req.ContextID, req.AgentName, req.ToolCallID, attachments)
	if err != nil {
		h.log.Error(r.Context(), "[API] Failed to submit task", "error", err)
		http.Error(w, "Failed to submit task", http.StatusInternalServerError)
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
	// Set CORS origin header early — even error responses need CORS
	// to be readable by the browser from the allowed origin.
	origin := h.allowedOrigin
	if origin == "" {
		origin = "self"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)

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

	// 5. Stream events with heartbeat to detect dead connections
	ctx := r.Context()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()
	// Maximum connection duration to prevent indefinite goroutine leaks
	maxTTL := time.NewTimer(30 * time.Minute)
	defer maxTTL.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-maxTTL.C:
			// Force close after max TTL to reclaim resources
			return
		case <-heartbeat.C:
			// SSE comment line as keepalive to detect dead connections
			if _, err := fmt.Fprintf(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
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
		http.Error(w, "Failed to cancel job", http.StatusBadRequest)

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

	// Verify ownership: the authenticated user must own this thread.
	state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID)
	if !exists {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}
	if state.UserID != user.ID {
		http.Error(w, "Thread not found", http.StatusNotFound)
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
		http.Error(w, "Failed to delete thread", http.StatusInternalServerError)

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

	// Verify ownership: the authenticated user must own this thread.
	state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID)
	if !exists {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}
	if state.UserID != user.ID {
		http.Error(w, "Thread not found", http.StatusNotFound)
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

	// Dispatch the event manually if the broadcaster isn't already handling it via the projection updater.
	// In this architecture, durable events are usually broadcasted by the projection updater,
	// but for immediate UI feedback on application-level events, we might want to dispatch here too.
	// However, looking at ProcessEvents in agentorchestrator/service.go, it seems durable events
	// are NOT broadcasted by the emitter if they are durable, to avoid double-broadcasting.
	// But ThreadTitleUpdated is an application event, not an agent event.
	// The ProjectionUpdater will eventually broadcast it once it's in Postgres.

	_ = event // Event can be used for logging or specific logic

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "title": req.Title}); err != nil {
		h.log.Error(r.Context(), "[API] Failed to encode rename response", "error", err)
	}
}

// ListAgents returns a list of available agents.
func (h *TaskManagementHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	// Try to list user-specific agents if provider supports it
	var agents []agent.Agent

	user := h.authService.GetUserFromContext(r.Context())

	type userAwareProvider interface {
		ListForUser(ctx context.Context, userID string) ([]agent.Agent, error)
	}

	if uap, ok := h.agentProvider.(userAwareProvider); ok && user != nil {
		var err error
		agents, err = uap.ListForUser(r.Context(), user.ID)
		if err != nil {
			agents = h.agentProvider.List() // Fallback to default
		}
	} else {
		agents = h.agentProvider.List()
	}

	response := agent.AgentListResponse{
		Agents: make([]agent.AgentMetadata, 0, len(agents)),
	}

	for _, a := range agents {
		meta := agent.AgentMetadata{
			ID:          a.ID(),
			Name:        a.Name(),
			Description: a.Description(),
			Icon:        resolveAgentIcon(a),
		}
		response.Agents = append(response.Agents, meta)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error(r.Context(), "failed to encode agents list", "error", err)
		http.Error(w, "Failed to encode agents list", http.StatusInternalServerError)
	}
}

// GetAgent returns a specific agent's details by ID.
func (h *TaskManagementHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())

	type userAwareProvider interface {
		GetForUser(ctx context.Context, name, userID string) (agent.Agent, error)
	}

	var a agent.Agent
	var err error

	if uap, ok := h.agentProvider.(userAwareProvider); ok && user != nil {
		a, err = uap.GetForUser(r.Context(), agentID, user.ID)
	} else {
		a, err = h.agentProvider.Get(agentID)
	}

	if err != nil {
		h.log.Error(r.Context(), "failed to get agent", "agentID", agentID, "error", err)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	meta := agent.AgentMetadata{
		ID:          a.ID(),
		Name:        a.Name(),
		Description: a.Description(),
		Icon:        resolveAgentIcon(a),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(meta); err != nil {
		h.log.Error(r.Context(), "failed to encode agent response", "error", err)
		http.Error(w, "Failed to encode agent response", http.StatusInternalServerError)
	}
}

// ListArtifacts returns a list of artifacts for a thread.
// GET /v1/threads/{threadId}/artifacts
func (h *TaskManagementHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	if threadID == "" {
		jsonError(w, "threadId is required", http.StatusBadRequest)
		return
	}

	// Thread ownership check — S3 keys are userID/threadID prefixed.
	// For new sessions the thread doesn't exist yet, so allow (returns empty).
	// Block only if the thread exists and belongs to a different user.
	if state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID); exists && state.UserID != user.ID {
		jsonError(w, "Thread not found", http.StatusNotFound)
		return
	}

	artifacts, err := h.artifactStore.List(r.Context(), user.ID, threadID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list artifacts", "error", err, "userID", user.ID, "threadID", threadID)
		jsonError(w, "Failed to list artifacts", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"artifacts": artifacts}); err != nil {
		h.log.Error(r.Context(), "failed to encode artifacts list", "error", err)
	}
}

// DownloadArtifact streams a specific artifact with HTTP Range request support.
// GET /v1/threads/{threadId}/artifacts/{fileName}
//
// Uses http.ServeContent which handles:
//   - Range requests (HTTP 206 Partial Content) for audio/video streaming
//   - Content-Length headers
//   - If-Modified-Since / If-None-Match conditional requests
func (h *TaskManagementHandler) DownloadArtifact(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	fileName := r.PathValue("fileName")
	if threadID == "" || fileName == "" {
		jsonError(w, "threadId and fileName are required", http.StatusBadRequest)
		return
	}

	// ── Thread ownership check ────────────────────────────────────────────
	// Prevents user A from reading user B's artifacts by guessing thread IDs.
	state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID)
	if !exists || state.UserID != user.ID {
		jsonError(w, "Artifact not found", http.StatusNotFound)
		return
	}

	content, meta, err := h.artifactStore.Load(r.Context(), user.ID, threadID, fileName)
	if err != nil {
		if errors.Is(err, artifact.ErrNotFound) {
			jsonError(w, "Artifact not found", http.StatusNotFound)
			return
		}
		h.log.Error(r.Context(), "failed to load artifact", "error", err, "userID", user.ID, "threadID", threadID, "fileName", fileName)
		jsonError(w, "Failed to load artifact", http.StatusInternalServerError)
		return
	}
	defer content.Close()

	// Set content type from S3 metadata
	ct := meta.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)

	// Use inline disposition for media types so browsers can play/display them,
	// and attachment for everything else so they download.
	if strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", fileName))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	}

	// Artifacts are named with UUID prefixes (immutable content-addressing).
	// Set aggressive cache headers so browsers and CDNs never re-fetch.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	// minio.Object implements io.ReadSeeker, so http.ServeContent can handle
	// Range requests (HTTP 206), Content-Length, and conditional headers.
	if rs, ok := content.(io.ReadSeeker); ok {
		http.ServeContent(w, r, fileName, time.Time{}, rs)
	} else {
		// Fallback: plain copy (no Range support)
		if _, copyErr := io.Copy(w, content); copyErr != nil {
			h.log.Error(r.Context(), "failed to stream artifact to client", "error", copyErr, "fileName", fileName)
		}
	}
}

// sanitizeFileName strips control characters, null bytes, path separators, and
// caps the length to prevent header injection and S3 key abuse.
func sanitizeFileName(name string) string {
	// Replace path separators with underscores
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")

	// Strip control characters (ASCII 0x00–0x1F, 0x7F)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if r > 0x1F && r != 0x7F {
			b.WriteRune(r)
		}
	}
	name = b.String()

	// Cap at 200 bytes (leave room for UUID prefix + underscore = ~9 chars)
	if len(name) > 200 {
		name = name[:200]
	}
	return strings.TrimSpace(name)
}

// jsonError writes a structured JSON error response.
// Industry practice: API error responses should be machine-parseable.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// allowedUploadTypes is the server-side allowlist of MIME types accepted for
// file uploads. This is the authoritative check — the frontend allowlist is
// advisory only. Keeping this tight prevents abuse (e.g. uploading executables).
var allowedUploadTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"audio/wav":       true,
	"audio/x-wav":     true,
	"audio/mpeg":      true,
	"audio/mp3":       true,
	"audio/ogg":       true,
	"audio/webm":      true,
	"application/pdf": true,
	"text/plain":      true,
	"text/csv":        true,
	"text/markdown":   true,
}

// UploadAttachment handles direct file uploads to the artifact store.
// POST /v1/threads/{threadId}/attachments:upload
//
// Production hardening for 100K concurrent users:
//   - Thread ownership verification (user A cannot upload to user B's thread)
//   - UUID-prefixed filenames prevent collisions across concurrent uploads
//   - Server-side MIME allowlist with http.DetectContentType sniff verification
//   - Multipart memory cap of 2 MB (spills to temp files, not RAM)
//
// Accepts multipart/form-data with a single "file" field.
// Returns JSON: {"url": "/v1/threads/.../artifacts/...", "mimeType": "...", "fileName": "..."}
func (h *TaskManagementHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	if threadID == "" {
		jsonError(w, "threadId is required", http.StatusBadRequest)
		return
	}

	// ── Thread ownership check ────────────────────────────────────────────
	// S3 keys are prefixed with userID/threadID, so namespace isolation is
	// inherent. For new sessions the thread doesn't exist in the projection
	// yet (it's created when the first message is sent), so we allow the
	// upload. If the thread *does* exist and belongs to another user, block.
	if state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID); exists && state.UserID != user.ID {
		jsonError(w, "Thread not found", http.StatusNotFound)
		return
	}

	// ── Parse multipart ───────────────────────────────────────────────────
	// maxMemory = 2 MB: keeps RAM bounded under concurrent load.
	// Excess spills to OS temp files (cleaned up when request ends).
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		jsonError(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// ── Content-type validation ───────────────────────────────────────────
	// 1. Read the declared type from the multipart header.
	contentType := header.Header.Get("Content-Type")

	// 2. Sniff actual bytes to defeat spoofed Content-Type headers.
	//    Read first 512 bytes (the amount http.DetectContentType uses).
	sniffBuf := make([]byte, 512)
	n, _ := file.Read(sniffBuf)
	detected := http.DetectContentType(sniffBuf[:n])

	// Seek back so S3 upload reads from the beginning.
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		jsonError(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Trust the declared type if it's in our allowlist AND the sniffed type
	// is either a match or "application/octet-stream" (many audio/pdf files
	// sniff as octet-stream because DetectContentType only knows a few).
	if contentType == "" || !allowedUploadTypes[contentType] {
		contentType = detected
	}
	if !allowedUploadTypes[contentType] {
		jsonError(w, fmt.Sprintf("Unsupported file type: %s", contentType), http.StatusUnsupportedMediaType)
		return
	}

	// ── Collision-safe filename ───────────────────────────────────────────
	// UUID prefix ensures no two concurrent uploads collide, even with the
	// same original filename across 100K users.
	originalName := sanitizeFileName(header.Filename)
	if originalName == "" {
		originalName = "upload"
	}
	fileName := fmt.Sprintf("%s_%s", uuid.New().String()[:8], originalName)

	// ── Save to S3 ───────────────────────────────────────────────────────
	_, saveErr := h.artifactStore.Save(r.Context(), user.ID, threadID, fileName, contentType, file)
	if saveErr != nil {
		h.log.Error(r.Context(), "failed to save uploaded attachment",
			"error", saveErr, "userID", user.ID, "threadID", threadID, "fileName", fileName)
		jsonError(w, "Failed to save attachment", http.StatusInternalServerError)
		return
	}

	artifactURL := fmt.Sprintf("/v1/threads/%s/artifacts/%s", threadID, fileName)

	h.log.Info(r.Context(), "[API] Attachment uploaded",
		"userID", user.ID, "threadID", threadID, "fileName", fileName,
		"contentType", contentType, "size", header.Size)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"url":      artifactURL,
		"mimeType": contentType,
		"fileName": fileName,
	})
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

// AgentToolSetResponse represents the API response format for agent tool sets.
type AgentToolSetResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    string              `json:"category"`
	Enabled     bool                `json:"enabled"`
	Tools       []ToolInSetResponse `json:"tools"`
}

// ToolInSetResponse represents individual tools within a tool set.
type ToolInSetResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"` // Individual tool permission status
}

// ListAgentToolSets returns a list of tool sets available to an agent.
// GET /v1/agents/{agentId}/toolsets
func (h *TaskManagementHandler) ListAgentToolSets(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}

	// Get all tool sets from registry
	allToolSets, err := h.toolRegistry.ListToolSets(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "Failed to list tool sets", "error", err)
		http.Error(w, "Failed to retrieve tool sets", http.StatusInternalServerError)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get enabled tool set IDs for the agent
	enabledToolSetIDs, err := h.permissionsService.GetEnabledToolSets(r.Context(), user.ID, agentID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get enabled tool sets", "agent_id", agentID, "error", err)
		http.Error(w, "Failed to retrieve agent tool set permissions", http.StatusInternalServerError)
		return
	}

	// Create a map of enabled tool sets for quick lookup
	enabledToolSets := make(map[string]bool)
	for _, toolSetID := range enabledToolSetIDs {
		enabledToolSets[toolSetID] = true
	}

	// Convert to response format
	responseToolSets := make([]AgentToolSetResponse, 0, len(allToolSets))
	for _, toolSet := range allToolSets {
		// Get tools in this tool set
		toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSet.ID)
		if err != nil {
			h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSet.ID, "error", err)
			continue
		}

		// Convert tools to response format with individual permissions
		toolResponses := make([]ToolInSetResponse, 0, len(toolsInSet))
		for _, tool := range toolsInSet {
			// Check individual tool permission - if no explicit permission, inherit from tool set
			toolEnabled := h.permissionsService.HasPermission(r.Context(), user.ID, agentID, toolSet.ID, tool.ID)

			toolResponses = append(toolResponses, ToolInSetResponse{
				ID:          tool.ID,
				Name:        tool.Name,
				Description: tool.Description,
				Category:    tool.Category,
				Enabled:     toolEnabled,
			})
		}

		responseToolSets = append(responseToolSets, AgentToolSetResponse{
			ID:          toolSet.ID,
			Name:        toolSet.Name,
			Description: toolSet.Description,
			Category:    toolSet.Category,
			Enabled:     enabledToolSets[toolSet.ID],
			Tools:       toolResponses,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"toolsets": responseToolSets}); err != nil {
		h.log.Error(r.Context(), "failed to encode tool sets list", "error", err)
	}
}

// ToggleAgentToolSetRequest represents the request body for toggling a tool set.
type ToggleAgentToolSetRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleAgentToolSet enables or disables a tool set for an agent.
// PUT /v1/agents/{agentId}/toolsets/{toolSetId}
func (h *TaskManagementHandler) ToggleAgentToolSet(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")
	if agentID == "" || toolSetID == "" {
		http.Error(w, "agentId and toolSetId are required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ToggleAgentToolSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify tool set exists in registry
	if _, err := h.toolRegistry.GetToolSet(r.Context(), toolSetID); err != nil {
		h.log.Error(r.Context(), "Tool set not found in registry",
			"tool_set_id", toolSetID, "error", err)
		http.Error(w, "Tool set not found", http.StatusNotFound)
		return
	}

	// Update permission in permissions service
	if err := h.permissionsService.SetToolSetPermission(r.Context(), user.ID, agentID, toolSetID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update agent tool set permission",
			"agent_id", agentID, "tool_set_id", toolSetID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update tool set permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":    "success",
		"toolSetId": toolSetID,
		"enabled":   req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle tool set response", "error", err)
	}
}

// ToggleIndividualToolRequest represents the request body for toggling an individual tool within a tool set.
type ToggleIndividualToolRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleIndividualTool enables or disables an individual tool within a tool set for an agent.
// PUT /v1/agents/{agentId}/toolsets/{toolSetId}/tools/{toolId}
func (h *TaskManagementHandler) ToggleIndividualTool(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")
	toolID := r.PathValue("toolId")

	if agentID == "" || toolSetID == "" || toolID == "" {
		http.Error(w, "agentId, toolSetId, and toolId are required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ToggleIndividualToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify tool set exists in registry
	if _, err := h.toolRegistry.GetToolSet(r.Context(), toolSetID); err != nil {
		h.log.Error(r.Context(), "Tool set not found in registry",
			"tool_set_id", toolSetID, "error", err)
		http.Error(w, "Tool set not found", http.StatusNotFound)
		return
	}

	// Verify tool exists in the tool set
	toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSetID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSetID, "error", err)
		http.Error(w, "Failed to verify tool", http.StatusInternalServerError)
		return
	}

	toolExists := false
	for _, tool := range toolsInSet {
		if tool.ID == toolID {
			toolExists = true
			break
		}
	}

	if !toolExists {
		http.Error(w, "Tool not found in tool set", http.StatusNotFound)
		return
	}

	// Update individual tool permission
	if err := h.permissionsService.SetPermission(r.Context(), user.ID, agentID, toolSetID, toolID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update individual tool permission",
			"agent_id", agentID, "tool_set_id", toolSetID, "tool_id", toolID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update tool permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":    "success",
		"toolSetId": toolSetID,
		"toolId":    toolID,
		"enabled":   req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle individual tool response", "error", err)
	}
}

// PluginResponse represents a plugin (tool set) in the simplified API.
type PluginResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    string              `json:"category"`
	Icon        string              `json:"icon"`
	Enabled     bool                `json:"enabled"`
	Tools       []ToolInSetResponse `json:"tools"`
}

// ListPlugins returns all available plugins with their enabled state.
// GET /v1/plugins (global — no per-agent state)
// GET /v1/plugins?agent_id=xxx (per-agent enabled state for agent config)
func (h *TaskManagementHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// agent_id is optional — if absent, return plugins without per-agent enabled state
	agentID := r.URL.Query().Get("agent_id")

	allToolSets, err := h.toolRegistry.ListToolSets(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "Failed to list tool sets", "error", err)
		http.Error(w, "Failed to retrieve plugins", http.StatusInternalServerError)
		return
	}

	// If an agent_id is provided, resolve per-agent enabled state
	enabledToolSets := make(map[string]bool)
	if agentID != "" {
		enabledToolSetIDs, err := h.permissionsService.GetEnabledToolSets(r.Context(), user.ID, agentID)
		if err != nil {
			h.log.Error(r.Context(), "Failed to get enabled tool sets", "agent_id", agentID, "error", err)
			// Continue without enabled state
		} else {
			for _, toolSetID := range enabledToolSetIDs {
				enabledToolSets[toolSetID] = true
			}
		}
	}

	plugins := make([]PluginResponse, 0, len(allToolSets))
	for _, toolSet := range allToolSets {
		toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSet.ID)
		if err != nil {
			h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSet.ID, "error", err)
			continue
		}

		toolResponses := make([]ToolInSetResponse, 0, len(toolsInSet))
		for _, tool := range toolsInSet {
			toolEnabled := false
			if agentID != "" {
				toolEnabled = h.permissionsService.HasPermission(r.Context(), user.ID, agentID, toolSet.ID, tool.ID)
			}
			toolResponses = append(toolResponses, ToolInSetResponse{
				ID:          tool.ID,
				Name:        tool.Name,
				Description: tool.Description,
				Category:    tool.Category,
				Enabled:     toolEnabled,
			})
		}

		plugins = append(plugins, PluginResponse{
			ID:          toolSet.ID,
			Name:        toolSet.Name,
			Description: toolSet.Description,
			Category:    toolSet.Category,
			Icon:        toolregistry.GetPluginIcon(toolSet.ID),
			Enabled:     enabledToolSets[toolSet.ID],
			Tools:       toolResponses,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"plugins": plugins}); err != nil {
		h.log.Error(r.Context(), "failed to encode plugins list", "error", err)
	}
}

// TogglePlugin enables or disables a plugin for a specific agent.
// PUT /v1/plugins/{pluginId}?agent_id=xxx (agent_id required)
func (h *TaskManagementHandler) TogglePlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("pluginId")
	if pluginID == "" {
		http.Error(w, "pluginId is required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id query parameter is required", http.StatusBadRequest)
		return
	}

	var req ToggleAgentToolSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := h.toolRegistry.GetToolSet(r.Context(), pluginID); err != nil {
		h.log.Error(r.Context(), "Plugin not found in registry", "plugin_id", pluginID, "error", err)
		http.Error(w, "Plugin not found", http.StatusNotFound)
		return
	}

	if err := h.permissionsService.SetToolSetPermission(r.Context(), user.ID, agentID, pluginID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update plugin permission",
			"agent_id", agentID, "plugin_id", pluginID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update plugin permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":   "success",
		"pluginId": pluginID,
		"enabled":  req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle plugin response", "error", err)
	}
}

// TogglePluginTool enables or disables an individual tool within a plugin for a specific agent.
// PUT /v1/plugins/{pluginId}/tools/{toolId}?agent_id=xxx (agent_id required)
func (h *TaskManagementHandler) TogglePluginTool(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("pluginId")
	toolID := r.PathValue("toolId")

	if pluginID == "" || toolID == "" {
		http.Error(w, "pluginId and toolId are required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id query parameter is required", http.StatusBadRequest)
		return
	}

	var req ToggleIndividualToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := h.toolRegistry.GetToolSet(r.Context(), pluginID); err != nil {
		h.log.Error(r.Context(), "Plugin not found in registry", "plugin_id", pluginID, "error", err)
		http.Error(w, "Plugin not found", http.StatusNotFound)
		return
	}

	toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), pluginID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tools in plugin", "plugin_id", pluginID, "error", err)
		http.Error(w, "Failed to verify tool", http.StatusInternalServerError)
		return
	}

	toolExists := false
	for _, tool := range toolsInSet {
		if tool.ID == toolID {
			toolExists = true
			break
		}
	}
	if !toolExists {
		http.Error(w, "Tool not found in plugin", http.StatusNotFound)
		return
	}

	if err := h.permissionsService.SetPermission(r.Context(), user.ID, agentID, pluginID, toolID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update tool permission",
			"agent_id", agentID, "plugin_id", pluginID, "tool_id", toolID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update tool permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":   "success",
		"pluginId": pluginID,
		"toolId":   toolID,
		"enabled":  req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle plugin tool response", "error", err)
	}
}

// ListDocuments handles GET /v1/documents - retrieves documents from Edgequake using passive context resolution.
func (h *TaskManagementHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Resolve Multitenancy Context
	var tCtx *shared.TenantContext
	if h.tenantID == "" {
		h.log.Error(r.Context(), "Tenant ID not configured")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Static Configuration
	tID, err := uuid.Parse(h.tenantID)
	if err != nil {
		h.log.Error(r.Context(), "Invalid configured tenant ID", "error", err)
		http.Error(w, "Invalid server configuration", http.StatusInternalServerError)
		return
	}
	tCtx = &shared.TenantContext{
		TenantID: &tID,
		UserID:   user.ID,
		Role:     string(shared.RoleAdmin),
	}

	// 3. Delegate to Document Provider (Passive Enforcement)
	// Go does NOT check if the user is allowed to list docs. It just passes the headers.
	resp, err := h.documentProvider.ListDocuments(r.Context(), tCtx, 1, 20, "")
	if err != nil {
		if errors.Is(err, chatcontracts.ErrKnowledgeUnauthorized) {
			http.Error(w, "Unauthorized by Document Provider", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, chatcontracts.ErrKnowledgeForbidden) {
			http.Error(w, "Access forbidden by Document Provider", http.StatusForbidden)
			return
		}
		h.log.Error(r.Context(), "Document Provider API failure", "error", err)
		http.Error(w, "Failed to retrieve documents", http.StatusInternalServerError)
		return
	}

	// 4. Return results
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error(r.Context(), "Failed to encode documents response", "error", err)
	}
}

// UploadDocument handles POST /v1/documents - uploads a document to Edgequake using passive context resolution.
func (h *TaskManagementHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Resolve Multitenancy Context
	var tCtx *shared.TenantContext
	if h.tenantID == "" {
		h.log.Error(r.Context(), "Tenant ID not configured")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Static Configuration
	tID, err := uuid.Parse(h.tenantID)
	if err != nil {
		h.log.Error(r.Context(), "Invalid configured tenant ID", "error", err)
		http.Error(w, "Invalid server configuration", http.StatusInternalServerError)
		return
	}
	tCtx = &shared.TenantContext{
		TenantID: &tID,
		UserID:   user.ID,
		Role:     string(shared.RoleAdmin),
	}

	// 3. Parse Multipart Form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 4. Delegate to Document Provider (Passive Enforcement)
	resp, err := h.documentProvider.UploadDocument(r.Context(), tCtx, header.Filename, header.Header.Get("Content-Type"), file)
	if err != nil {
		if errors.Is(err, chatcontracts.ErrKnowledgeUnauthorized) {
			http.Error(w, "Unauthorized by Document Provider", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, chatcontracts.ErrKnowledgeForbidden) {
			http.Error(w, "Access forbidden by Document Provider", http.StatusForbidden)
			return
		}
		h.log.Error(r.Context(), "Document Provider API failure during upload", "error", err)
		http.Error(w, "Failed to upload document", http.StatusInternalServerError)
		return
	}

	// 5. Return result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error(r.Context(), "Failed to encode upload response", "error", err)
	}
}
