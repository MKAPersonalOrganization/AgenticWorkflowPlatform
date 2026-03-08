package skillstore

import (
	"encoding/json"
	"net/http"

	"school-with-ai/foundation/logger"
	"school-with-ai/shared"
)

// permissionHandler encapsulates HTTP handlers for skill-agent permission management.
type permissionHandler struct {
	log *logger.Logger
	svc *service
}

func newPermissionHandler(log *logger.Logger, svc *service) *permissionHandler {
	return &permissionHandler{log: log, svc: svc}
}

// ─── Request/Response Types ─────────────────────────────────────────────────

type setPermissionRequest struct {
	Enabled bool `json:"enabled"`
}

type permissionResponse struct {
	AgentID string `json:"agent_id"`
	SkillID string `json:"skill_id"`
	Enabled bool   `json:"enabled"`
}

type permissionListResponse struct {
	Permissions []permissionResponse `json:"permissions"`
}

type agentSkillResponse struct {
	SkillID     string `json:"skill_id"`
	SkillName   string `json:"skill_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Icon        string `json:"icon"`
}

type agentSkillsListResponse struct {
	Skills []agentSkillResponse `json:"skills"`
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// listBySkill handles GET /v1/skills/{id}/permissions
func (h *permissionHandler) listBySkill(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	skillID := r.PathValue("id")
	if skillID == "" {
		http.Error(w, "Skill ID is required", http.StatusBadRequest)
		return
	}

	perms, err := h.svc.listPermissionsBySkill(r.Context(), user.ID, skillID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list skill permissions", "error", err)
		http.Error(w, "Failed to list permissions", http.StatusInternalServerError)
		return
	}

	resp := permissionListResponse{
		Permissions: make([]permissionResponse, 0, len(perms)),
	}
	for _, p := range perms {
		resp.Permissions = append(resp.Permissions, permissionResponse{
			AgentID: p.AgentID,
			SkillID: p.SkillID,
			Enabled: p.Enabled,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// setPermission handles PUT /v1/skills/{id}/permissions/{agentId}
func (h *permissionHandler) setPermission(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	skillID := r.PathValue("id")
	agentID := r.PathValue("agentId")
	if skillID == "" || agentID == "" {
		http.Error(w, "Skill ID and Agent ID are required", http.StatusBadRequest)
		return
	}

	var req setPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Enabled {
		if err := h.svc.grantPermission(r.Context(), user.ID, agentID, skillID); err != nil {
			h.log.Error(r.Context(), "failed to grant skill permission", "error", err)
			http.Error(w, "Failed to grant permission", http.StatusInternalServerError)
			return
		}
	} else {
		if err := h.svc.revokePermission(r.Context(), user.ID, agentID, skillID); err != nil {
			h.log.Error(r.Context(), "failed to revoke skill permission", "error", err)
			http.Error(w, "Failed to revoke permission", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"agent_id": agentID,
		"skill_id": skillID,
		"enabled":  req.Enabled,
		"status":   "updated",
	})
}

// listByAgent handles GET /v1/agents/{agentId}/skills
func (h *permissionHandler) listByAgent(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	skills, err := h.svc.listSkillsForAgent(r.Context(), user.ID, agentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list agent skills", "error", err)
		http.Error(w, "Failed to list agent skills", http.StatusInternalServerError)
		return
	}

	resp := agentSkillsListResponse{
		Skills: make([]agentSkillResponse, 0, len(skills)),
	}
	for _, s := range skills {
		resp.Skills = append(resp.Skills, agentSkillResponse{
			SkillID:     s.ID,
			SkillName:   s.Name,
			Description: s.Description,
			Category:    s.Category,
			Icon:        skillIconOrDefault(s.Icon),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
