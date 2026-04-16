package threadhandler

import (
	"aicompass/foundation/logger"
	"aicompass/modules/chat/projection/agui"
	"aicompass/modules/chat/thread"
	"aicompass/modules/chat/threadmgmt"
	"aicompass/foundation/auth"
	"encoding/json"
	"errors"
	"net/http"
)

// Handler handles thread-related HTTP endpoints.
type Handler struct {
	log            *logger.Logger
	aguiProjection agui.AguiProjection
	threadMgmt     *threadmgmt.Manager
}

// New creates a new thread handler.
func New(
	log *logger.Logger,
	aguiProjection agui.AguiProjection,
	threadMgmt *threadmgmt.Manager,
) *Handler {
	return &Handler{
		log:            log,
		aguiProjection: aguiProjection,
		threadMgmt:     threadMgmt,
	}
}

// DeleteThread deletes a thread.
func (h *Handler) DeleteThread(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)

		return
	}

	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)

		return
	}

	// Verify ownership: the authenticated user must own this thread.
	state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID)
	if !exists {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}
	if state.UserID != user.ID {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	h.log.Info(r.Context(), "Deleting thread", "userID", user.ID, "threadID", threadID)

	// Delete thread using thread management manager
	_, err := h.threadMgmt.DeleteThread(r.Context(), threadID)
	if err != nil {
		if errors.Is(err, thread.ErrThreadNotFound) {
			http.Error(w, "Thread not found", http.StatusNotFound)

			return
		}
		http.Error(w, "Failed to delete thread", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	if err != nil {
		h.log.Error(r.Context(), "[API] Failed to encode delete response", "error", err)
	}
}

// UpdateThreadTitleRequest represents request body for renaming a thread.
type UpdateThreadTitleRequest struct {
	Title string `json:"title"`
}

// UpdateThreadTitle renames a thread.
// PATCH /api/agui/threads/{threadId}
func (h *Handler) UpdateThreadTitle(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	threadID := r.PathValue("threadId")
	if threadID == "" {
		http.Error(w, "threadId is required", http.StatusBadRequest)
		return
	}

	// Verify ownership: the authenticated user must own this thread.
	state, exists := h.aguiProjection.GetThreadState(r.Context(), threadID)
	if !exists {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}
	if state.UserID != user.ID {
		http.Error(w, "Thread not found", http.StatusNotFound)
		return
	}

	var req UpdateThreadTitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	h.log.Info(r.Context(), "Renaming thread", "userID", user.ID, "threadID", threadID, "newTitle", req.Title)

	event, err := h.threadMgmt.UpdateThreadTitle(r.Context(), threadID, req.Title)
	if err != nil {
		if errors.Is(err, thread.ErrThreadNotFound) {
			http.Error(w, "Thread not found", http.StatusNotFound)
			return
		}
		h.log.Error(r.Context(), "failed to update thread title", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_ = event // Event can be used for logging or specific logic

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "title": req.Title}); err != nil {
		h.log.Error(r.Context(), "[API] Failed to encode rename response", "error", err)
	}
}
