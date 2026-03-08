package skillstore

import (
	"encoding/json"
	"net/http"
	"strings"

	"school-with-ai/foundation/logger"
	"school-with-ai/shared"
)

// skillHandler encapsulates HTTP handlers for user skill CRUD operations.
type skillHandler struct {
	log *logger.Logger
	svc *service
}

func newSkillHandler(log *logger.Logger, svc *service) *skillHandler {
	return &skillHandler{log: log, svc: svc}
}

// ─── Request/Response Types ─────────────────────────────────────────────────

type createSkillRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
	Category     string `json:"category"`
	Icon         string `json:"icon"`
}

type updateSkillRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
	Category     string `json:"category"`
	Icon         string `json:"icon"`
}

type skillResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Instructions    string `json:"instructions"`
	Category        string `json:"category"`
	Icon            string `json:"icon"`
	IsCustom        bool   `json:"is_custom"`
	SourceCatalogID string `json:"source_catalog_id,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type skillListResponse struct {
	Skills []skillResponse `json:"skills"`
}

// defaultSkillIconSVG is the fallback icon (zap) for skills without a valid SVG icon.
var defaultSkillIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z"/></svg>`

func skillIconOrDefault(icon string) string {
	if strings.HasPrefix(icon, "<svg") {
		return icon
	}
	return defaultSkillIconSVG
}

func toSkillResponse(s *UserSkill) skillResponse {
	return skillResponse{
		ID:              s.ID,
		Name:            s.Name,
		Description:     s.Description,
		Instructions:    s.Instructions,
		Category:        s.Category,
		Icon:            skillIconOrDefault(s.Icon),
		IsCustom:        s.IsCustom,
		SourceCatalogID: s.SourceCatalogID,
		CreatedAt:       s.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// create handles POST /v1/skills
func (h *skillHandler) create(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req createSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Skill name is required", http.StatusBadRequest)
		return
	}
	if req.Instructions == "" {
		http.Error(w, "Skill instructions are required", http.StatusBadRequest)
		return
	}

	skill := &UserSkill{
		UserID:       user.ID,
		Name:         req.Name,
		Description:  req.Description,
		Instructions: req.Instructions,
		Category:     req.Category,
		Icon:         skillIconOrDefault(req.Icon),
		IsCustom:     true,
	}

	if err := h.svc.create(r.Context(), skill); err != nil {
		h.log.Error(r.Context(), "failed to create skill", "error", err)
		// Check for limit error
		if strings.Contains(err.Error(), "limit reached") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "limit_reached",
				"detail": err.Error(),
			})
			return
		}
		http.Error(w, "Failed to create skill", http.StatusInternalServerError)
		return
	}

	resp := toSkillResponse(skill)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// update handles PUT /v1/skills/{id}
func (h *skillHandler) update(w http.ResponseWriter, r *http.Request) {
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

	var req updateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Skill name is required", http.StatusBadRequest)
		return
	}
	if req.Instructions == "" {
		http.Error(w, "Skill instructions are required", http.StatusBadRequest)
		return
	}

	// Fetch existing to preserve icon if not provided
	existing, err := h.svc.getByID(r.Context(), skillID, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "skill not found", "error", err)
		http.Error(w, "Skill not found", http.StatusNotFound)
		return
	}

	iconValue := existing.Icon
	if req.Icon != "" {
		iconValue = req.Icon
	}

	skill := &UserSkill{
		ID:           skillID,
		UserID:       user.ID,
		Name:         req.Name,
		Description:  req.Description,
		Instructions: req.Instructions,
		Category:     req.Category,
		Icon:         iconValue,
	}

	if err := h.svc.update(r.Context(), skill); err != nil {
		h.log.Error(r.Context(), "failed to update skill", "error", err)
		http.Error(w, "Failed to update skill", http.StatusInternalServerError)
		return
	}

	updated, err := h.svc.getByID(r.Context(), skillID, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to fetch updated skill", "error", err)
		http.Error(w, "Failed to fetch updated skill", http.StatusInternalServerError)
		return
	}

	resp := toSkillResponse(updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// delete handles DELETE /v1/skills/{id}
func (h *skillHandler) delete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.svc.delete(r.Context(), skillID, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to delete skill", "error", err)
		http.Error(w, "Failed to delete skill", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// list handles GET /v1/skills
func (h *skillHandler) list(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	skills, err := h.svc.listByUser(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list skills", "error", err)
		http.Error(w, "Failed to list skills", http.StatusInternalServerError)
		return
	}

	resp := skillListResponse{
		Skills: make([]skillResponse, 0, len(skills)),
	}
	for i := range skills {
		resp.Skills = append(resp.Skills, toSkillResponse(&skills[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// get handles GET /v1/skills/{id}
func (h *skillHandler) get(w http.ResponseWriter, r *http.Request) {
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

	skill, err := h.svc.getByID(r.Context(), skillID, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get skill", "error", err)
		http.Error(w, "Skill not found", http.StatusNotFound)
		return
	}

	resp := toSkillResponse(skill)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// search handles GET /v1/skills/search?q=...
func (h *skillHandler) search(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		h.list(w, r)
		return
	}

	skills, err := h.svc.search(r.Context(), user.ID, query)
	if err != nil {
		h.log.Error(r.Context(), "failed to search skills", "error", err)
		http.Error(w, "Failed to search skills", http.StatusInternalServerError)
		return
	}

	resp := skillListResponse{
		Skills: make([]skillResponse, 0, len(skills)),
	}
	for i := range skills {
		resp.Skills = append(resp.Skills, toSkillResponse(&skills[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
