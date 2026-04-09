package taskmgmt

import (
	"aicompass/foundation/agent"
	"aicompass/modules/chat/agentorchestrator"
	"aicompass/modules/chat/taskmanagement"
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
	ToolCallID  string           `json:"toolCallId,omitempty"`  // Optional: correlates this message as an answer to a tool's elicitation
	Attachments []FileAttachment `json:"attachments,omitempty"` // Optional: attached files
}

// SendMessage handles POST /v1/message:send — runs the agent inline and streams
// AGUI events back as SSE on the same HTTP response.
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

	// Generate IDs
	contextID := req.ContextID
	if contextID == "" {
		contextID = fmt.Sprintf("thread_%s_%d", user.ID, time.Now().UnixNano())
	}
	messageID := agent.GenerateID()

	h.log.Info(r.Context(), "[API] Streaming chat request",
		"userID", user.ID,
		"contextID", contextID,
		"numAttachments", len(req.Attachments))

	// Convert file attachments
	var attachments []agentorchestrator.FileAttachment
	for _, att := range req.Attachments {
		attachments = append(attachments, agentorchestrator.FileAttachment{
			URL:      att.URL,
			MimeType: att.MimeType,
			FileName: att.FileName,
		})
	}

	// Set SSE headers — must happen before any writes commit the status code.
	origin := h.allowedOrigin
	if origin == "" {
		origin = "self"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Disable any reverse-proxy response buffering (nginx, Cloudflare, etc.)
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Per-write deadline pattern: set a short deadline before every individual
	// write so zombie clients are detected quickly.
	rc := http.NewResponseController(w)

	// Opt into full-duplex mode so HTTP/2's response buffering is bypassed.
	// Without this, Go's HTTP/2 framer coalesces small SSE writes into a
	// single DATA frame, causing tokens to arrive in bursts instead of
	// streaming smoothly. On HTTP/1.1 this is a no-op.
	if err := rc.EnableFullDuplex(); err != nil {
		h.log.Info(r.Context(), "[API] EnableFullDuplex not supported, SSE may buffer on HTTP/2", "error", err)
	}

	// Send the contextId as the first SSE event so the frontend knows the threadID.
	_ = rc.SetWriteDeadline(time.Now().Add(h.sse.WriteTimeout))
	if err := WriteSSEEvent(w, map[string]string{"type": "SESSION_START", "contextId": contextID}, ""); err != nil {
		return
	}
	flusher.Flush()

	// Build orchestrator request
	execReq := agentorchestrator.RunExecutionRequest{
		ThreadID:    contextID,
		UserID:      user.ID,
		AgentName:   req.AgentName,
		Message:     req.Message,
		MessageID:   messageID,
		ToolCallID:  req.ToolCallID,
		Attachments: attachments,
	}

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

		aguiEvents := taskmanagement.ToAguiEvents(event)
		for _, ae := range aguiEvents {
			if err := WriteSSEEvent(w, ae, ""); err != nil {
				h.log.Info(r.Context(), "[API] Write failed, client disconnected")
				return
			}
		}
		flusher.Flush()
	}
}
