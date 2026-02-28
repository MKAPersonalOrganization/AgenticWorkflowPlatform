package taskmanagement

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator"

	"github.com/hibiken/asynq"
)

// Task type constants.
const (
	TypeProcessChat = "chat:process"
)

// FileAttachment represents a file sent with a chat message (image, audio, PDF, etc.).
type FileAttachment struct {
	Data     string `json:"data,omitempty"`     // Base64 data URL — legacy path (data:image/png;base64,...)
	URL      string `json:"url,omitempty"`      // Pre-uploaded artifact URL (e.g. /v1/threads/.../artifacts/file.wav)
	MimeType string `json:"mimeType,omitempty"` // e.g. "image/png", "audio/wav"
	FileName string `json:"fileName,omitempty"` // Original filename
}

// ProcessChatPayload contains the data needed to process a chat message.
type ProcessChatPayload struct {
	JobID       string           `json:"job_id"`
	ThreadID    string           `json:"thread_id"`
	MessageID   string           `json:"message_id"`
	Message     string           `json:"message"`
	UserID      string           `json:"user_id"`
	AgentName   string           `json:"agent_name,omitempty"`
	ToolCallID  string           `json:"tool_call_id,omitempty"` // Set when this message is an answer to a tool's elicitation
	Attachments []FileAttachment `json:"attachments,omitempty"`  // Attached files (base64 data URLs)
}

// NewProcessChatTask creates a new task for processing a chat message.
func NewProcessChatTask(payload ProcessChatPayload) (*asynq.Task, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return asynq.NewTask(TypeProcessChat, payloadBytes,
		asynq.MaxRetry(3),             // Retry up to 3 times on failure
		asynq.Timeout(5*time.Minute),  // 5 minute timeout for LLM calls
		asynq.TaskID(payload.JobID),   // Use job ID as task ID for deduplication
		asynq.Retention(24*time.Hour), // Keep completed tasks for 24 hours
	), nil
}

// ChatWorker processes chat messages.
type ChatWorker struct {
	log                  *logger.Logger
	orchestrationService *Service
}

// NewChatWorker creates a new chat worker.
func NewChatWorker(
	log *logger.Logger,
	orchestrationService *Service,
) *ChatWorker {
	return &ChatWorker{
		log:                  log,
		orchestrationService: orchestrationService,
	}
}

// ProcessTask implements asynq.Handler interface.
func (w *ChatWorker) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var payload ProcessChatPayload
	err := json.Unmarshal(t.Payload(), &payload)
	if err != nil {
		return fmt.Errorf("json.Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}

	w.log.Info(ctx, "[WORKER] Picked up job from Redis queue",
		"jobID", payload.JobID,
		"threadID", payload.ThreadID,
		"message", payload.Message)

	// Process the chat message
	err = w.processChat(ctx, payload)
	if err != nil {
		return err
	}

	w.log.Info(ctx, "[WORKER] Job completed successfully",
		"jobID", payload.JobID,
		"threadID", payload.ThreadID,
		"note", "Events streamed to client via SSE")

	return nil
}

// processChat processes a single chat message using the orchestration service.
func (w *ChatWorker) processChat(ctx context.Context, payload ProcessChatPayload) error {
	// Check if context was cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("task cancelled: %w", ctx.Err())
	}

	// Map directly to RunExecutionRequest (no intermediate struct)
	req := agentorchestrator.RunExecutionRequest{
		ThreadID:    payload.ThreadID,
		UserID:      payload.UserID,
		AgentName:   payload.AgentName,
		Message:     payload.Message,
		MessageID:   payload.MessageID,
		ToolCallID:  payload.ToolCallID,
		Attachments: toOrchestratorAttachments(payload.Attachments),
	}

	return w.orchestrationService.HandleUserMessage(ctx, req)
}

// toOrchestratorAttachments converts taskmanagement attachments to orchestrator attachments.
func toOrchestratorAttachments(atts []FileAttachment) []agentorchestrator.FileAttachment {
	if len(atts) == 0 {
		return nil
	}
	result := make([]agentorchestrator.FileAttachment, len(atts))
	for i, att := range atts {
		result[i] = agentorchestrator.FileAttachment{
			Data:     att.Data,
			URL:      att.URL,
			MimeType: att.MimeType,
			FileName: att.FileName,
		}
	}
	return result
}
