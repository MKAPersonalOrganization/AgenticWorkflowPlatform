package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"school-with-ai/internal/agui"
	"school-with-ai/internal/session"
	"school-with-ai/internal/tools"
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

		// Get conversation history
		messages := store.GetMessages(threadID)

		// Create streaming handler
		streamHandler := openaiservice.NewStreamHandler(aiClient, store, w, flusher)

		// Get available tools
		availableTools := tools.GetAllTools()

		// Handle OpenAI streaming
		ctx := context.Background()
		if err := streamHandler.HandleStream(ctx, threadID, messageID, messages, availableTools); err != nil {
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

		// Check if tool calls were made and continue conversation
		updatedMessages := store.GetMessages(threadID)
		hasToolCalls := false
		for _, msg := range updatedMessages {
			if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
				hasToolCalls = true
				break
			}
		}

		// If tool calls were made, continue conversation with results
		if hasToolCalls {
			// Generate new message ID for final response
			finalMessageID := agui.GenerateMessageID()

			// Call OpenAI again with tool results in conversation
			if err := streamHandler.HandleStream(ctx, threadID, finalMessageID, updatedMessages, availableTools); err != nil {
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

		// Auto-generate thread title if still "New Chat"
		thread, exists := store.GetThread(threadID)
		if exists && thread.Title == "New Chat" && len(thread.Messages) >= 2 {
			// Generate title synchronously before finishing the run
			messages := store.GetMessages(threadID)
			if title, err := aiClient.GenerateThreadTitle(ctx, messages); err == nil && title != "" {
				store.UpdateThreadTitle(threadID, title)

				// Send CUSTOM event to notify frontend of title update
				customEvent := map[string]any{
					"type":      "CUSTOM",
					"name":      "THREAD_TITLE_UPDATED",
					"value": map[string]string{
						"threadId": threadID,
						"title":    title,
					},
					"timestamp": time.Now().UnixMilli(),
				}
				agui.WriteSSEEvent(w, customEvent)
				flusher.Flush()
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
