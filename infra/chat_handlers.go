package infra

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/agui"
	"school-with-ai/services/agent"
	"school-with-ai/services/llm"
)

// Handler encapsulates dependencies for HTTP handlers
type Handler struct {
	store      agent.Repository
	provider   llm.Provider
	queue      domain.JobQueue
	workerPool *WorkerPool
}

// NewHandler creates a new HTTP handler with the given dependencies
func NewHandler(store agent.Repository, provider llm.Provider, queue domain.JobQueue, workerPool *WorkerPool) *Handler {
	return &Handler{
		store:      store,
		provider:   provider,
		queue:      queue,
		workerPool: workerPool,
	}
}

// SendMessage handles the /send endpoint by enqueuing a job
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
	// Get thread ID and message from query parameters
	threadID := r.URL.Query().Get("threadId")
	if threadID == "" {
		threadID = fmt.Sprintf("thread_%d", time.Now().UnixNano())
	}
	message := r.URL.Query().Get("message")
	if message == "" {
		message = "Hello"
	}

	// Create job
	jobID := agent.GenerateID()
	messageID := agui.GenerateMessageID()

	job := &domain.Job{
		ID:        jobID,
		ThreadID:  threadID,
		MessageID: messageID,
		Message:   message,
		Status:    domain.JobStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Enqueue job
	if err := h.queue.Enqueue(job); err != nil {
		http.Error(w, fmt.Sprintf("Failed to enqueue job: %v", err), http.StatusInternalServerError)
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
func (h *Handler) StreamJobEvents(w http.ResponseWriter, r *http.Request) {
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

	// Wait for job to be picked up by a worker
	ctx := r.Context()
	var eventBuffer *EventBuffer

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
			eventBuffer, exists = h.workerPool.GetEventBuffer(jobID)
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

func (h *Handler) translateAndWriteEvent(w http.ResponseWriter, flusher http.Flusher, event any) error {
	var aguiEvent any

	// Translate domain events to agui events
	switch ev := event.(type) {
	case *agent.TextMessageStartEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.TextMessageStartEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TEXT_MESSAGE_START",
				Timestamp: &timestamp,
			},
			MessageID: ev.MessageID,
			Role:      ev.Role,
		}

	case *agent.TextMessageContentEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.TextMessageContentEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TEXT_MESSAGE_CONTENT",
				Timestamp: &timestamp,
			},
			MessageID: ev.MessageID,
			Delta:     ev.Delta,
		}

	case *agent.TextMessageEndEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.TextMessageEndEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TEXT_MESSAGE_END",
				Timestamp: &timestamp,
			},
			MessageID: ev.MessageID,
		}

	case *agent.ToolCallStartEvent:
		timestamp := time.Now().UnixMilli()
		parentMessageID := ev.ParentMessageID
		aguiEvent = &agui.ToolCallStartEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_START",
				Timestamp: &timestamp,
			},
			ToolCallID:      ev.ToolCallID,
			ToolCallName:    ev.ToolCallName,
			ParentMessageID: &parentMessageID,
		}

	case *agent.ToolCallArgsEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.ToolCallArgsEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_ARGS",
				Timestamp: &timestamp,
			},
			ToolCallID: ev.ToolCallID,
			Delta:      ev.Delta,
		}

	case *agent.ToolCallEndEvent:
		timestamp := time.Now().UnixMilli()
		aguiEvent = &agui.ToolCallEndEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_END",
				Timestamp: &timestamp,
			},
			ToolCallID: ev.ToolCallID,
		}

	case *agent.ToolCallResultEvent:
		timestamp := time.Now().UnixMilli()
		role := ev.Role
		aguiEvent = &agui.ToolCallResultEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "TOOL_CALL_RESULT",
				Timestamp: &timestamp,
			},
			MessageID:  ev.MessageID,
			ToolCallID: ev.ToolCallID,
			Content:    ev.Content,
			Role:       &role,
		}

	default:
		// If it's already an agui event, pass through
		aguiEvent = event
	}

	if err := agui.WriteSSEEvent(w, aguiEvent); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// GetActiveJobs retrieves all active jobs for a thread
func (h *Handler) GetActiveJobs(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("jobId")
	if jobID == "" {
		http.Error(w, "jobId is required", http.StatusBadRequest)
		return
	}

	// Attempt to cancel the job in queue (if pending)
	if err := h.queue.CancelJob(jobID); err != nil {
		if err == ErrJobNotFound {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		// If not pending, might be processing - try worker pool
		if !h.workerPool.CancelJob(jobID) {
			http.Error(w, fmt.Sprintf("Failed to cancel job: %v", err), http.StatusBadRequest)
			return
		}
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
func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	messages := h.store.GetMessages(threadID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(messages); err != nil {
		http.Error(w, "Failed to encode messages", http.StatusInternalServerError)
		return
	}
}

// GetThreads retrieves all threads
func (h *Handler) GetThreads(w http.ResponseWriter, r *http.Request) {
	threads := h.store.GetAllThreads()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(threads); err != nil {
		http.Error(w, "Failed to encode threads", http.StatusInternalServerError)
		return
	}
}

// DeleteThread deletes a thread
func (h *Handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	deleted := h.store.DeleteThread(threadID)
	if !deleted {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// emitMessagesSnapshot converts session messages to AG-UI messages and emits MESSAGES_SNAPSHOT event
func (h *Handler) emitMessagesSnapshot(w http.ResponseWriter, flusher http.Flusher, threadID string) error {
	sessionMessages := h.store.GetMessages(threadID)

	// Convert session.Message to agui.Message
	aguiMessages := make([]agui.Message, 0, len(sessionMessages))
	for _, msg := range sessionMessages {
		aguiMsg := convertSessionMessageToAGUI(msg)
		aguiMessages = append(aguiMessages, aguiMsg)
	}

	// Emit MESSAGES_SNAPSHOT event
	timestamp := time.Now().UnixMilli()
	messagesSnapshotEvent := &agui.MessagesSnapshotEvent{
		BaseEvent: agui.BaseEvent{
			Type:      "MESSAGES_SNAPSHOT",
			Timestamp: &timestamp,
		},
		Messages: aguiMessages,
	}

	if err := agui.WriteSSEEvent(w, messagesSnapshotEvent); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// convertSessionMessageToAGUI converts a session.Message to agui.Message (discriminated union)
func convertSessionMessageToAGUI(msg domain.Message) agui.Message {
	switch msg.Role {
	case "user":
		return agui.UserMessage{
			BaseMessage: agui.BaseMessage{
				ID:      msg.ID,
				Role:    msg.Role,
				Content: &msg.Content,
			},
			Content: msg.Content,
		}
	case "assistant":
		assistantMsg := agui.AssistantMessage{
			BaseMessage: agui.BaseMessage{
				ID:      msg.ID,
				Role:    msg.Role,
				Content: &msg.Content,
			},
		}

		// Convert tool calls if present
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]agui.ToolCall, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, agui.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: agui.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				})
			}
			assistantMsg.ToolCalls = toolCalls
		}

		return assistantMsg
	case "tool":
		return agui.ToolMessage{
			ID:         msg.ID,
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
	case "system":
		return agui.SystemMessage{
			BaseMessage: agui.BaseMessage{
				ID:      msg.ID,
				Role:    msg.Role,
				Content: &msg.Content,
			},
			Content: msg.Content,
		}
	case "developer":
		return agui.DeveloperMessage{
			BaseMessage: agui.BaseMessage{
				ID:      msg.ID,
				Role:    msg.Role,
				Content: &msg.Content,
			},
			Content: msg.Content,
		}
	default:
		// Default to user message for unknown roles
		return agui.UserMessage{
			BaseMessage: agui.BaseMessage{
				ID:      msg.ID,
				Role:    msg.Role,
				Content: &msg.Content,
			},
			Content: msg.Content,
		}
	}
}
