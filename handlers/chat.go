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
	"school-with-ai/services/llm"
	"school-with-ai/services/streaming"
)

// HandleSendMessage handles the /send endpoint with SSE streaming
func HandleSendMessage(store *session.Store, provider llm.Provider) http.HandlerFunc {
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
				errorTimestamp := time.Now().UnixMilli()
				errorCode := "INTERNAL_ERROR"
				errorEvent := &agui.RunErrorEvent{
					BaseEvent: agui.BaseEvent{
						Type:      "RUN_ERROR",
						Timestamp: &errorTimestamp,
					},
					Message: fmt.Sprintf("%v", r),
					Code:    &errorCode,
				}
				agui.WriteSSEEvent(w, errorEvent)
				flusher.Flush()
			}
		}()

		// Send RUN_STARTED event
		runStartedTimestamp := time.Now().UnixMilli()
		runStartedEvent := &agui.RunStartedEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "RUN_STARTED",
				Timestamp: &runStartedTimestamp,
			},
			ThreadID: threadID,
			RunID:    runID,
		}
		if err := agui.WriteSSEEvent(w, runStartedEvent); err != nil {
			return
		}
		flusher.Flush()

		// Generate unique message ID for assistant response
		messageID := agui.GenerateMessageID()

		// Get conversation history
		messages := store.GetMessages(threadID)

		// Create user message handler
		messageHandler := streaming.NewUserMessageHandler(provider, store, w, flusher)

		// Get available tools
		availableTools := tools.GetAllTools()

		// Process user message and stream response
		ctx := context.Background()
		if err := messageHandler.HandleStream(ctx, threadID, messageID, messages, availableTools); err != nil {
			// Emit RUN_ERROR event on failure
			errorTimestamp := time.Now().UnixMilli()
			errorCode := "LLM_ERROR"
			errorEvent := &agui.RunErrorEvent{
				BaseEvent: agui.BaseEvent{
					Type:      "RUN_ERROR",
					Timestamp: &errorTimestamp,
				},
				Message: err.Error(),
				Code:    &errorCode,
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
			// Emit MESSAGES_SNAPSHOT after tool results
			if err := emitMessagesSnapshot(w, flusher, threadID, store); err != nil {
				return
			}

			// Generate new message ID for final response
			finalMessageID := agui.GenerateMessageID()

			// Process again with tool results in conversation
			if err := messageHandler.HandleStream(ctx, threadID, finalMessageID, updatedMessages, availableTools); err != nil {
				errorTimestamp := time.Now().UnixMilli()
				errorCode := "LLM_ERROR"
				errorEvent := &agui.RunErrorEvent{
					BaseEvent: agui.BaseEvent{
						Type:      "RUN_ERROR",
						Timestamp: &errorTimestamp,
					},
					Message: err.Error(),
					Code:    &errorCode,
				}
				agui.WriteSSEEvent(w, errorEvent)
				flusher.Flush()
				return
			}
		}

		// Emit final MESSAGES_SNAPSHOT with complete conversation
		if err := emitMessagesSnapshot(w, flusher, threadID, store); err != nil {
			return
		}

		// Auto-generate thread title if still "New Chat"
		thread, exists := store.GetThread(threadID)
		if exists && thread.Title == "New Chat" && len(thread.Messages) >= 2 {
			// Generate simple title from first user message
			messages := store.GetMessages(threadID)
			if len(messages) > 0 {
				// Take first 50 characters of first user message as title
				title := messages[0].Content
				if len(title) > 50 {
					title = title[:50] + "..."
				}
				store.UpdateThreadTitle(threadID, title)

				// Send CUSTOM event to notify frontend of title update
				customTimestamp := time.Now().UnixMilli()
				customEvent := &agui.CustomEvent{
					BaseEvent: agui.BaseEvent{
						Type:      "CUSTOM",
						Timestamp: &customTimestamp,
					},
					Name: "THREAD_TITLE_UPDATED",
					Value: map[string]string{
						"threadId": threadID,
						"title":    title,
					},
				}
				agui.WriteSSEEvent(w, customEvent)
				flusher.Flush()
			}
		}

		// Send RUN_FINISHED event
		runFinishedTimestamp := time.Now().UnixMilli()
		runFinishedEvent := &agui.RunFinishedEvent{
			BaseEvent: agui.BaseEvent{
				Type:      "RUN_FINISHED",
				Timestamp: &runFinishedTimestamp,
			},
			ThreadID: threadID,
			RunID:    runID,
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

// emitMessagesSnapshot converts session messages to AG-UI messages and emits MESSAGES_SNAPSHOT event
func emitMessagesSnapshot(w http.ResponseWriter, flusher http.Flusher, threadID string, store *session.Store) error {
	sessionMessages := store.GetMessages(threadID)

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
func convertSessionMessageToAGUI(msg session.Message) agui.Message {
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
