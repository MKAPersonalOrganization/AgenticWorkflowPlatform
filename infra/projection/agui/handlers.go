package agui

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"school-with-ai/infra/projection/whiteboard"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ThreadSummary represents a summary of a thread for list views
type ThreadSummary struct {
	ID                string               `json:"id"`
	Title             string               `json:"title"`
	Messages          []aguievents.Message `json:"messages"`
	CreatedAt         time.Time            `json:"createdAt"`
	UpdatedAt         time.Time            `json:"updatedAt"`
	WhiteboardContent string               `json:"whiteboardContent"`
	CurrentInstructor string               `json:"currentInstructor,omitempty"`
}

// AguiProjectionHandler provides HTTP endpoints for querying the AGUI projection.
type AguiProjectionHandler struct {
	projection      AguiProjection
	whiteboardStore whiteboard.WhiteboardStore
}

// NewAguiProjectionHandler creates a new handler for AGUI projection queries.
func NewAguiProjectionHandler(projection AguiProjection, whiteboardStore whiteboard.WhiteboardStore) *AguiProjectionHandler {
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
	state.WhiteboardContent = h.whiteboardStore.Get(threadID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
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

	messages := h.projection.GetMessages(threadID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(messages); err != nil {
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
	if err := json.NewEncoder(w).Encode(toolCalls); err != nil {
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
	if err := json.NewEncoder(w).Encode(activity); err != nil {
		slog.Error("Failed to encode agent activity", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// ListThreads returns all thread IDs in the projection.
// GET /api/agui/threads
func (h *AguiProjectionHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	threadIDs := h.projection.ListThreads()

	threads := make([]ThreadSummary, 0, len(threadIDs))
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

		threads = append(threads, ThreadSummary{
			ID:                threadID,
			Title:             state.Title,
			Messages:          state.Messages,
			CreatedAt:         state.CreatedAt,
			UpdatedAt:         state.UpdatedAt,
			WhiteboardContent: whiteboardContent,
			CurrentInstructor: currentInstructor,
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

	w.Header().Set("Content-Type", "application/json")
	if !exists {
		if err := json.NewEncoder(w).Encode(nil); err != nil {
			slog.Error("Failed to encode null active run", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	if err := json.NewEncoder(w).Encode(run); err != nil {
		slog.Error("Failed to encode active run", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
