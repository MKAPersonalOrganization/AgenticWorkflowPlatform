package infra

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"school-with-ai/services/sdk"
)

// AguiProjectionHandler provides HTTP endpoints for querying the AGUI projection.
type AguiProjectionHandler struct {
	projection sdk.AguiProjection
}

// NewAguiProjectionHandler creates a new handler for AGUI projection queries.
func NewAguiProjectionHandler(projection sdk.AguiProjection) *AguiProjectionHandler {
	return &AguiProjectionHandler{
		projection: projection,
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
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"threads": threadIDs,
	}); err != nil {
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
