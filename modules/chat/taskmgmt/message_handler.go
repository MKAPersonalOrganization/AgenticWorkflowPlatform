package taskmgmt

import (
	"encoding/json"
	"fmt"
	"net/http"

	"school-with-ai/modules/chat/taskmanagement"
)

// FileAttachment represents a file sent with a message (image, audio, PDF, etc.).
type FileAttachment struct {
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
	Attachments []FileAttachment `json:"attachments,omitempty"` // Optional: attached files
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
