package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"school-with-ai/internal/agui"
	"school-with-ai/internal/session"
	openaiservice "school-with-ai/services/openai"
)

// HandleSendMessage handles the /send endpoint with SSE streaming
func HandleSendMessage(store *session.Store, aiClient *openaiservice.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// Get thread ID and message from query parameters
		threadID := r.URL.Query().Get("threadId")
		if threadID == "" {
			threadID = fmt.Sprintf("thread_%d", time.Now().UnixNano())
		}
		message := r.URL.Query().Get("message")
		if message == "" {
			message = "Hello"
		}

		// Ensure thread exists
		store.GetOrCreateThread(threadID)

		// Store user message
		userMsgID := agui.GenerateMessageID()
		store.AddMessage(threadID, session.Message{
			ID:        userMsgID,
			Role:      "user",
			Content:   message,
			Timestamp: time.Now(),
		})

		// Generate unique run ID for this execution
		runID := agui.GenerateRunID()

		// Error handling with defer to catch panics
		defer func() {
			if r := recover(); r != nil {
				errorEvent := &agui.RunErrorEvent{
					Type:      "RUN_ERROR",
					RunID:     runID,
					Code:      "INTERNAL_ERROR",
					Message:   fmt.Sprintf("%v", r),
					Timestamp: time.Now().UnixMilli(),
				}
				agui.WriteSSEEvent(w, errorEvent)
				flusher.Flush()
			}
		}()

		// Send RUN_STARTED event
		runStartedEvent := &agui.RunStartedEvent{
			Type:      "RUN_STARTED",
			ThreadID:  threadID,
			RunID:     runID,
			Timestamp: time.Now().UnixMilli(),
		}
		if err := agui.WriteSSEEvent(w, runStartedEvent); err != nil {
			return
		}
		flusher.Flush()

		// Generate unique message ID for assistant response
		messageID := agui.GenerateMessageID()

		// Check if OpenAI client is available
		if aiClient == nil {
			// Fallback to echo response if no API key
			sendEchoResponse(w, flusher, threadID, messageID, message, store)
		} else {
			// Get conversation history
			messages := store.GetMessages(threadID)

			// Create streaming handler
			streamHandler := openaiservice.NewStreamHandler(aiClient, store, w, flusher)

			// Handle OpenAI streaming
			ctx := context.Background()
			if err := streamHandler.HandleStream(ctx, threadID, messageID, messages); err != nil {
				// Emit RUN_ERROR event on failure
				errorEvent := &agui.RunErrorEvent{
					Type:      "RUN_ERROR",
					RunID:     runID,
					Code:      "OPENAI_ERROR",
					Message:   err.Error(),
					Timestamp: time.Now().UnixMilli(),
				}
				agui.WriteSSEEvent(w, errorEvent)
				flusher.Flush()
				return
			}
		}

		// Send RUN_FINISHED event
		runFinishedEvent := &agui.RunFinishedEvent{
			Type:      "RUN_FINISHED",
			ThreadID:  threadID,
			RunID:     runID,
			Timestamp: time.Now().UnixMilli(),
		}
		if err := agui.WriteSSEEvent(w, runFinishedEvent); err != nil {
			return
		}
		flusher.Flush()
	}
}

// HandleGetMessages retrieves all messages for a thread
func HandleGetMessages(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threadID := r.PathValue("threadId")
		if threadID == "" {
			http.Error(w, "threadId is required", http.StatusBadRequest)
			return
		}

		messages := store.GetMessages(threadID)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(messages); err != nil {
			http.Error(w, "Failed to encode messages", http.StatusInternalServerError)
			return
		}
	}
}

// sendEchoResponse sends a simple echo response (fallback when OpenAI is not available)
func sendEchoResponse(w http.ResponseWriter, flusher http.Flusher, threadID, messageID, message string, store *session.Store) {
	// Send TEXT_MESSAGE_START event
	startEvent := &agui.TextMessageStartEvent{
		Type:      "TEXT_MESSAGE_START",
		MessageID: messageID,
		Role:      "assistant",
		Timestamp: time.Now().UnixMilli(),
	}
	agui.WriteSSEEvent(w, startEvent)
	flusher.Flush()

	// Generate echo response
	response := "Echo (OpenAI not configured): " + message

	// Send content as a single chunk
	contentEvent := &agui.TextMessageContentEvent{
		Type:      "TEXT_MESSAGE_CONTENT",
		MessageID: messageID,
		Delta:     response,
		Timestamp: time.Now().UnixMilli(),
	}
	agui.WriteSSEEvent(w, contentEvent)
	flusher.Flush()

	// Store assistant message
	store.AddMessage(threadID, session.Message{
		ID:        messageID,
		Role:      "assistant",
		Content:   response,
		Timestamp: time.Now(),
	})

	// Send TEXT_MESSAGE_END event
	endEvent := &agui.TextMessageEndEvent{
		Type:      "TEXT_MESSAGE_END",
		MessageID: messageID,
		Timestamp: time.Now().UnixMilli(),
	}
	agui.WriteSSEEvent(w, endEvent)
	flusher.Flush()
}

// HandleGetThreads retrieves all threads
func HandleGetThreads(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threads := store.GetAllThreads()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(threads); err != nil {
			http.Error(w, "Failed to encode threads", http.StatusInternalServerError)
			return
		}
	}
}

// UpdateThreadTitleRequest represents the request body for updating thread title
type UpdateThreadTitleRequest struct {
	Title string `json:"title"`
}

// HandleUpdateThreadTitle updates a thread's title
func HandleUpdateThreadTitle(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		store.UpdateThreadTitle(threadID, req.Title)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}

// HandleDeleteThread deletes a thread
func HandleDeleteThread(store *session.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threadID := r.PathValue("threadId")
		if threadID == "" {
			http.Error(w, "threadId is required", http.StatusBadRequest)
			return
		}

		deleted := store.DeleteThread(threadID)
		if !deleted {
			http.Error(w, "Thread not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}
}
