package agui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// convertToAGUISDKFormat converts AguiMessage to aguievents.Message (AG-UI SDK format)
// This ensures all REST API endpoints return the same format as SSE (unified data format)
func convertToAGUISDKFormat(projectionMessages []AguiMessage) []aguievents.Message {
	aguiMessages := make([]aguievents.Message, 0, len(projectionMessages))
	for _, msg := range projectionMessages {
		aguiMsg := aguievents.Message{
			ID:   msg.ID,
			Role: msg.Role,
		}
		if msg.Content != "" {
			aguiMsg.Content = &msg.Content
		}
		if msg.AgentName != "" {
			aguiMsg.Name = &msg.AgentName
		}
		// Convert tool calls from flat to nested format
		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]aguievents.ToolCall, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				toolCalls[i] = aguievents.ToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: aguievents.Function{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				}
			}
			aguiMsg.ToolCalls = toolCalls
		}
		// Convert tool call result
		if msg.ToolCall != nil {
			aguiMsg.ToolCallID = &msg.ToolCall.ID
		}
		aguiMessages = append(aguiMessages, aguiMsg)
	}
	return aguiMessages
}

// AguiProjectionHandler provides HTTP endpoints for querying the AGUI projection.
type AguiProjectionHandler struct {
	projection      AguiProjection
	whiteboardStore interface{ Get(threadID string) string }
}

// NewAguiProjectionHandler creates a new handler for AGUI projection queries.
func NewAguiProjectionHandler(projection AguiProjection, whiteboardStore interface{ Get(threadID string) string }) *AguiProjectionHandler {
	return &AguiProjectionHandler{
		projection:      projection,
		whiteboardStore: whiteboardStore,
	}
}

// GetThreadState returns the complete UI state for a thread.
// GET /api/agui/threads/{threadId}
func (h *AguiProjectionHandler) GetThreadState(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	state, exists := h.projection.GetThreadState(threadID)
	if !exists {
		http.Error(w, "thread not found", http.StatusNotFound)
		return
	}

	// Add whiteboard content from dedicated projection
	whiteboardContent := h.whiteboardStore.Get(threadID)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"threadId":          state.ThreadID,
		"title":             state.Title,
		"messages":          state.Messages,
		"activeRun":         state.ActiveRun,
		"toolCalls":         state.ToolCalls,
		"agentActivity":     state.AgentActivity,
		"whiteboardContent": whiteboardContent,
		"createdAt":         state.CreatedAt,
		"updatedAt":         state.UpdatedAt,
		"version":           state.Version,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Failed to encode thread state", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetMessages returns all UI-formatted messages for a thread.
// GET /api/agui/threads/{threadId}/messages
func (h *AguiProjectionHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	projectionMessages := h.projection.GetMessages(threadID)
	aguiMessages := convertToAGUISDKFormat(projectionMessages)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": aguiMessages,
	}); err != nil {
		slog.Error("Failed to encode messages", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetToolCalls returns all tool calls for a thread.
// GET /api/agui/threads/{threadId}/tool-calls
func (h *AguiProjectionHandler) GetToolCalls(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	toolCalls := h.projection.GetToolCalls(threadID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"toolCalls": toolCalls,
	}); err != nil {
		slog.Error("Failed to encode tool calls", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetAgentActivity returns agent activity summary for a thread.
// GET /api/agui/threads/{threadId}/agent-activity
func (h *AguiProjectionHandler) GetAgentActivity(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	activity := h.projection.GetAgentActivity(threadID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"agentActivity": activity,
	}); err != nil {
		slog.Error("Failed to encode agent activity", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// ListThreads returns all thread IDs in the projection.
// GET /api/agui/threads
func (h *AguiProjectionHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	threadIDs := h.projection.ListThreads()

	// Build thread summaries with full state
	threads := make([]map[string]interface{}, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		state, exists := h.projection.GetThreadState(threadID)
		if !exists {
			continue
		}

		// Get whiteboard content
		whiteboardContent := h.whiteboardStore.Get(threadID)

		// Determine current instructor from agent activity
		var currentInstructor string
		if len(state.AgentActivity) > 0 {
			// Get the most recently active agent
			var latestTime time.Time
			for agentName, activity := range state.AgentActivity {
				if activity.LastActiveAt.After(latestTime) {
					latestTime = activity.LastActiveAt
					currentInstructor = agentName
				}
			}
		}

		// Convert messages to AG-UI SDK format for consistency
		aguiMessages := convertToAGUISDKFormat(state.Messages)

		threads = append(threads, map[string]interface{}{
			"id":                threadID,
			"title":             state.Title,
			"messages":          aguiMessages,
			"createdAt":         state.CreatedAt,
			"updatedAt":         state.UpdatedAt,
			"whiteboardContent": whiteboardContent,
			"currentInstructor": currentInstructor,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(threads); err != nil {
		slog.Error("Failed to encode thread list", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetActiveRun returns information about the currently active run for a thread.
// GET /api/agui/threads/{threadId}/active-run
func (h *AguiProjectionHandler) GetActiveRun(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	run, exists := h.projection.GetActiveRun(threadID)
	if !exists {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"activeRun": nil,
		}); err != nil {
			slog.Error("Failed to encode null active run", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"activeRun": run,
	}); err != nil {
		slog.Error("Failed to encode active run", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
