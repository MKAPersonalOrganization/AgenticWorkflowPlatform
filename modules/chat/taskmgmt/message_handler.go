package taskmgmt

import (
	"aicompass/foundation/agent"
	"aicompass/modules/chat/agentorchestrator"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FileAttachment represents a file sent with a message (image, audio, PDF, etc.).
type FileAttachment struct {
	URL      string `json:"url,omitempty"`      // Pre-uploaded artifact URL (/v1/threads/.../artifacts/...)
	MimeType string `json:"mimeType,omitempty"` // e.g. "image/png", "audio/wav"
	FileName string `json:"fileName,omitempty"` // Original filename
}

// SendMessageRequest represents the chat request.
type SendMessageRequest struct {
	ContextID   string           `json:"contextId,omitempty"`   // Optional: existing context (threadID)
	Message     string           `json:"message"`               // The message/input for the task
	AgentName   string           `json:"agentName,omitempty"`   // Optional: specific agent to target
	Attachments []FileAttachment `json:"attachments,omitempty"` // Optional: attached files
}

// ConfirmToolRequest represents a tool confirmation response (Allow/Deny).
type ConfirmToolRequest struct {
	ContextID  string `json:"contextId"`           // Required: thread/chat session ID
	ToolCallID string `json:"toolCallId"`          // Required: which tool call to confirm
	Confirmed  bool   `json:"confirmed"`           // User's choice: true = Allow, false = Deny
	AgentName  string `json:"agentName,omitempty"` // Optional: specific agent (auto-detected if empty)
}

// ProvideInputRequest represents a custom user input response (io.Read).
type ProvideInputRequest struct {
	ContextID  string `json:"contextId"`           // Required: thread/chat session ID
	ToolCallID string `json:"toolCallId"`          // Required: which tool call requested input
	Input      string `json:"input"`               // Required: the user's text response
	AgentName  string `json:"agentName,omitempty"` // Optional: specific agent
}

// SendMessage handles POST /v1/message:send — runs the agent inline and streams
// AGUI events back as SSE on the same HTTP response.
func (h *TaskManagementHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Message == "" && len(req.Attachments) == 0 {
		http.Error(w, "Message or attachments required", http.StatusBadRequest)
		return
	}

	contextID := req.ContextID
	if contextID == "" {
		contextID = fmt.Sprintf("thread_%s_%d", user.ID, time.Now().UnixNano())
	}

	h.log.Info(r.Context(), "[API] Streaming chat request",
		"userID", user.ID,
		"contextID", contextID,
		"numAttachments", len(req.Attachments))

	var attachments []agentorchestrator.FileAttachment
	for _, att := range req.Attachments {
		attachments = append(attachments, agentorchestrator.FileAttachment{
			URL:      att.URL,
			MimeType: att.MimeType,
			FileName: att.FileName,
		})
	}

	execReq := agentorchestrator.RunExecutionRequest{
		ThreadID:    contextID,
		UserID:      user.ID,
		AgentName:   req.AgentName,
		Message:     req.Message,
		MessageID:   agent.GenerateID(),
		Attachments: attachments,
	}

	h.streamRun(w, r, execReq)
}

// ConfirmTool handles POST /v1/tool:confirm — responds to a tool confirmation request.
// Streams AGUI events back as SSE on the same HTTP response.
func (h *TaskManagementHandler) ConfirmTool(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ConfirmToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ContextID == "" || req.ToolCallID == "" {
		http.Error(w, "contextId and toolCallId are required", http.StatusBadRequest)
		return
	}

	h.log.Info(r.Context(), "[API] Tool confirmation request",
		"userID", user.ID,
		"contextID", req.ContextID,
		"toolCallId", req.ToolCallID,
		"confirmed", req.Confirmed)

	// Format binary confirmation as JSON (read by ADK's confirmation logic)
	message := fmt.Sprintf(`{"confirmed":%v}`, req.Confirmed)

	execReq := agentorchestrator.RunExecutionRequest{
		ThreadID:   req.ContextID,
		UserID:     user.ID,
		AgentName:  req.AgentName,
		Message:    message,
		MessageID:  agent.GenerateID(),
		ToolCallID: req.ToolCallID,
	}

	h.streamRun(w, r, execReq)
}

// ProvideInput handles POST /v1/tool:provide_input — responds to an io.Read request.
// Streams AGUI events back as SSE on the same HTTP response.
func (h *TaskManagementHandler) ProvideInput(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ProvideInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ContextID == "" || req.ToolCallID == "" || req.Input == "" {
		http.Error(w, "contextId, toolCallId, and input are required", http.StatusBadRequest)
		return
	}

	h.log.Info(r.Context(), "[API] Custom input response",
		"userID", user.ID,
		"contextID", req.ContextID,
		"toolCallId", req.ToolCallID)

	execReq := agentorchestrator.RunExecutionRequest{
		ThreadID:   req.ContextID,
		UserID:     user.ID,
		AgentName:  req.AgentName,
		Message:    req.Input,
		MessageID:  agent.GenerateID(),
		ToolCallID: req.ToolCallID,
		UserInput:  req.Input,
	}

	h.streamRun(w, r, execReq)
}

// streamRun executes a run and streams SSE events to the client.
// This is the shared streaming logic used by both SendMessage and ConfirmTool.
func (h *TaskManagementHandler) streamRun(w http.ResponseWriter, r *http.Request, execReq agentorchestrator.RunExecutionRequest) {
	// Set SSE headers — must happen before any writes commit the status code.
	origin := h.allowedOrigin
	if origin == "" {
		origin = "self"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	rc := http.NewResponseController(w)

	// Opt into full-duplex mode for smooth streaming on HTTP/2.
	if err := rc.EnableFullDuplex(); err != nil {
		h.log.Info(r.Context(), "[API] EnableFullDuplex not supported, SSE may buffer on HTTP/2", "error", err)
	}

	// Send the contextId as the first SSE event so the frontend knows the threadID.
	_ = rc.SetWriteDeadline(time.Now().Add(h.sse.WriteTimeout))
	if err := WriteSSEEvent(w, map[string]string{"type": "SESSION_START", "contextId": execReq.ThreadID}, ""); err != nil {
		return
	}
	flusher.Flush()

	// Execute the run and stream events inline
	for event, err := range h.orchestrator.ExecuteRun(r.Context(), execReq) {
		// Check if context was cancelled OR write to client failed
		writeErr := rc.SetWriteDeadline(time.Now().Add(h.sse.WriteTimeout))
		if writeErr != nil || r.Context().Err() != nil {
			h.log.Info(r.Context(), "[API] Client disconnected, aborting stream")
			return
		}

		if err != nil {
			h.log.Error(r.Context(), "[API] Run execution error", "error", err)
			_ = WriteSSEEvent(w, map[string]any{"type": "RUN_ERROR", "message": err.Error()}, "")
			flusher.Flush()
			return
		}

		aguiEvents := ToAguiEvents(event)
		for _, ae := range aguiEvents {
			if err := WriteSSEEvent(w, ae, ""); err != nil {
				h.log.Info(r.Context(), "[API] Write failed, client disconnected")
				return
			}
		}
		flusher.Flush()
	}
}
