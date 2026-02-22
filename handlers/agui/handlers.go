package agui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"school-with-ai/domain"
	"school-with-ai/infra/projection/agui"
	"school-with-ai/services"
)

// AguiProjectionHandler provides HTTP endpoints for querying the AGUI projection.
type AguiProjectionHandler struct {
	projection        agui.AguiProjection
	projectionService ProjectionService
}

// ProjectionService provides business logic for projections.
type ProjectionService interface {
	ListThreadSummaries() []services.ThreadSummary
}

// NewAguiProjectionHandler creates a new handler for AGUI projection queries.
func NewAguiProjectionHandler(projection agui.AguiProjection, projectionService ProjectionService) *AguiProjectionHandler {
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

	h.writeThreadResponse(w, state)
}

// GetMessages returns all UI-formatted messages for a thread.
// GET /api/agui/threads/{threadId}/messages.
func (h *AguiProjectionHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)

		return
	}

	// Get full thread state including token usage
	state, exists := h.projection.GetThreadState(threadID)
	if !exists {
		http.Error(w, "thread not found", http.StatusNotFound)
		return
	}

	h.writeThreadResponse(w, state)
}

// writeThreadResponse writes the standard thread state response.
func (h *AguiProjectionHandler) writeThreadResponse(w http.ResponseWriter, state *agui.AguiThreadState) {
	response := map[string]interface{}{
		"messages": state.Messages,
	}

	if state.TokenUsage != nil {
		response["tokenUsage"] = state.TokenUsage
	}

	if state.ToolStates != nil {
		response["toolStates"] = state.ToolStates
	}

	// Encode to buffer first to avoid partial writes with incorrect status
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(response); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(buf.Bytes())
}

// ListThreads returns paginated thread summaries, sorted by most recent.
// GET /api/agui/threads?cursor=<timestamp>&limit=<int>.
func (h *AguiProjectionHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	// Get user from auth context
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query params
	cursor := r.URL.Query().Get("cursor")
	limit := 10 // default
	const maxLimit = 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit < 1 {
			http.Error(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
		limit = parsedLimit
		if limit > maxLimit {
			limit = maxLimit
		}
	}

	// Parse optional search query
	query := r.URL.Query().Get("q")

	// Build request and fetch paginated threads
	req := agui.ThreadListRequest{
		UserID: user.ID,
		Cursor: cursor,
		Limit:  limit,
		Query:  query,
	}

	response, err := h.projection.ListThreads(r.Context(), req)
	if err != nil {
		http.Error(w, "invalid cursor format", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
