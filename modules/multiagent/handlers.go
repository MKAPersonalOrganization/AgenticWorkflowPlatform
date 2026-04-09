package multiagent

import (
	"aicompass/foundation/logger"
	"aicompass/shared"
	"encoding/json"
	"net/http"
)

// ──────────────────────────────────────────────────────────────────────────────
// HTTP Handlers
// ──────────────────────────────────────────────────────────────────────────────

type workflowHandler struct {
	log *logger.Logger
	svc *service
}

func newWorkflowHandler(log *logger.Logger, svc *service) *workflowHandler {
	return &workflowHandler{log: log, svc: svc}
}

// list returns all workflows for the authenticated user.
func (h *workflowHandler) list(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	workflows, err := h.svc.listByUser(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "listing workflows", "error", err)
		http.Error(w, `{"error":"failed to list workflows"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"workflows": workflows,
	})
}

// create creates a new workflow definition.
func (h *workflowHandler) create(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Name        string       `json:"name"`
		Description string       `json:"description"`
		Icon        string       `json:"icon"`
		Graph       WorkflowNode `json:"graph"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	wf := &WorkflowDefinition{
		UserID:      user.ID,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Graph:       req.Graph,
	}

	if err := h.svc.create(r.Context(), wf); err != nil {
		h.log.Error(r.Context(), "creating workflow", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, wf)
}

// get returns a single workflow by ID.
func (h *workflowHandler) get(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"workflow ID is required"}`, http.StatusBadRequest)
		return
	}

	wf, err := h.svc.getByID(r.Context(), id, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "getting workflow", "error", err)
		http.Error(w, `{"error":"workflow not found"}`, http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, wf)
}

// update modifies an existing workflow.
func (h *workflowHandler) update(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"workflow ID is required"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Name        string       `json:"name"`
		Description string       `json:"description"`
		Icon        string       `json:"icon"`
		Graph       WorkflowNode `json:"graph"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	wf := &WorkflowDefinition{
		ID:          id,
		UserID:      user.ID,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		Graph:       req.Graph,
	}

	if err := h.svc.update(r.Context(), wf); err != nil {
		h.log.Error(r.Context(), "updating workflow", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, wf)
}

// deleteWF removes a workflow definition.
func (h *workflowHandler) deleteWF(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"workflow ID is required"}`, http.StatusBadRequest)
		return
	}

	if err := h.svc.delete(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "deleting workflow", "error", err)
		http.Error(w, `{"error":"failed to delete workflow"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
