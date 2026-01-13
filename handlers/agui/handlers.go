package agui

import (
	"encoding/json"
	"net/http"

	"school-with-ai/domain"
	"school-with-ai/services"
)

// AguiProjectionHandler provides HTTP endpoints for querying the AGUI projection.
type AguiProjectionHandler struct {
	projection        domain.AguiProjection
	projectionService ProjectionService
}

// ProjectionService provides business logic for projections.
type ProjectionService interface {
	ListThreadSummaries() []services.ThreadSummary
}

// NewAguiProjectionHandler creates a new handler for AGUI projection queries.
func NewAguiProjectionHandler(projection domain.AguiProjection, projectionService ProjectionService) *AguiProjectionHandler {
	return &AguiProjectionHandler{
		projection:        projection,
		projectionService: projectionService,
	}
}

// GetThreadState returns the complete UI state for a thread.
// GET /api/agui/threads/{threadId}.
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
	err := json.NewEncoder(w).Encode(state)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetMessages returns all UI-formatted messages for a thread.
// GET /api/agui/threads/{threadId}/messages.
func (h *AguiProjectionHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)

		return
	}

	messages := h.projection.GetMessages(threadID)

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(messages)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// ListThreads returns all thread summaries, sorted by most recent.
// GET /api/agui/threads.
func (h *AguiProjectionHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	threads := h.projectionService.ListThreadSummaries()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(threads)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// GetActiveRun returns information about the currently active run for a thread.
// GET /api/agui/threads/{threadId}/active-run.
func (h *AguiProjectionHandler) GetActiveRun(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)

		return
	}

	run, exists := h.projection.GetActiveRun(threadID)

	w.Header().Set("Content-Type", "application/json")
	if !exists {
		err := json.NewEncoder(w).Encode(nil)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}

		return
	}

	err := json.NewEncoder(w).Encode(run)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
