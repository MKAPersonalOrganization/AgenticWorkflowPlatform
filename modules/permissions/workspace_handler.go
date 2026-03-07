package permissions

import (
	"encoding/json"
	"net/http"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/permissions/workspace"
	"school-with-ai/shared"

	"github.com/google/uuid"
)

type workspacePermHandler struct {
	log              *logger.Logger
	workspacePermSvc *workspace.Service
	workspaceProv    WorkspaceProvider
	agentProvider    agent.AgentProvider
	tenantID         string
}

// newWorkspacePermHandler creates a new workspace permissions handler.
func newWorkspacePermHandler(
	log *logger.Logger,
	workspacePermSvc *workspace.Service,
	workspaceProv WorkspaceProvider,
	agentProvider agent.AgentProvider,
	tenantID string,
) *workspacePermHandler {
	return &workspacePermHandler{
		log:              log,
		workspacePermSvc: workspacePermSvc,
		workspaceProv:    workspaceProv,
		agentProvider:    agentProvider,
		tenantID:         tenantID,
	}
}

// AgentWorkspaceResponse represents a workspace with its permission state for an agent.
type AgentWorkspaceResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Slug        string `json:"slug"`
	IsActive    bool   `json:"is_active"`
	Enabled     bool   `json:"enabled"`
	DocCount    int    `json:"doc_count,omitempty"`
}

// ListAgentWorkspaces returns all workspaces with permission state for an agent.
// GET /v1/agents/{agentId}/workspaces
func (h *workspacePermHandler) ListAgentWorkspaces(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}

	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Build tenant context
	tenantUUID, err := uuid.Parse(h.tenantID)
	if err != nil {
		h.log.Error(r.Context(), "Invalid tenant ID", "error", err)
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	tCtx := &shared.TenantContext{
		TenantID: &tenantUUID,
		UserID:   user.ID,
		Role:     "user",
	}

	// List workspaces from provider
	workspacesResp, err := h.workspaceProv.ListWorkspaces(r.Context(), tCtx, 1, 100)
	if err != nil {
		h.log.Error(r.Context(), "Failed to list workspaces from provider", "error", err)
		http.Error(w, "Failed to retrieve workspaces", http.StatusInternalServerError)
		return
	}

	// Get permissions for this agent
	permissions, err := h.workspacePermSvc.ListPermissionsByAgent(r.Context(), user.ID, agentID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to list workspace permissions", "agent_id", agentID, "error", err)
		http.Error(w, "Failed to retrieve permissions", http.StatusInternalServerError)
		return
	}

	// Build permission lookup
	permMap := make(map[string]bool)
	for _, perm := range permissions {
		permMap[perm.WorkspaceID] = perm.Enabled
	}

	// Merge workspaces with permission state
	responseWorkspaces := make([]AgentWorkspaceResponse, 0, len(workspacesResp.Items))
	for _, ws := range workspacesResp.Items {
		desc := ""
		if ws.Description != nil {
			desc = *ws.Description
		}
		responseWorkspaces = append(responseWorkspaces, AgentWorkspaceResponse{
			ID:          ws.ID.String(),
			Name:        ws.Name,
			Description: desc,
			Slug:        ws.Slug,
			IsActive:    ws.IsActive,
			Enabled:     permMap[ws.ID.String()],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"workspaces": responseWorkspaces}); err != nil {
		h.log.Error(r.Context(), "failed to encode workspaces list", "error", err)
	}
}

// ToggleAgentWorkspaceRequest represents the request body for toggling a workspace attachment.
type ToggleAgentWorkspaceRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleAgentWorkspace enables or disables a workspace for an agent.
// PUT /v1/agents/{agentId}/workspaces/{workspaceId}
func (h *workspacePermHandler) ToggleAgentWorkspace(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	workspaceID := r.PathValue("workspaceId")
	if agentID == "" || workspaceID == "" {
		http.Error(w, "agentId and workspaceId are required", http.StatusBadRequest)
		return
	}

	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req ToggleAgentWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.workspacePermSvc.SetPermission(r.Context(), user.ID, agentID, workspaceID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update workspace permission",
			"agent_id", agentID, "workspace_id", workspaceID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update workspace permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":      "success",
		"workspaceId": workspaceID,
		"enabled":     req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle workspace response", "error", err)
	}
}
