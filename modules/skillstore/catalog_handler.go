package skillstore

import (
	"encoding/json"
	"net/http"
	"strconv"

	"school-with-ai/foundation/logger"
	"school-with-ai/shared"
)

// catalogHandler encapsulates HTTP handlers for the shared skill catalog (marketplace).
type catalogHandler struct {
	log       *logger.Logger
	catalog   SkillCatalogRepository
	skillRepo UserSkillRepository
}

func newCatalogHandler(log *logger.Logger, catalog SkillCatalogRepository, skillRepo UserSkillRepository) *catalogHandler {
	return &catalogHandler{log: log, catalog: catalog, skillRepo: skillRepo}
}

// catalogEntryResponse represents a single catalog entry in API responses.
type catalogEntryResponse struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Instructions string   `json:"instructions"`
	Category     string   `json:"category"`
	Tags         []string `json:"tags"`
	AuthorName   string   `json:"author_name"`
	Icon         string   `json:"icon"`
	Featured     bool     `json:"featured"`
	AddCount     int64    `json:"add_count"`
}

// catalogListResponse is the response for listing/searching catalog entries.
type catalogListResponse struct {
	Entries         []catalogEntryResponse `json:"entries"`
	Total           int                    `json:"total"`
	Categories      []string               `json:"categories,omitempty"`
	AddedCatalogIDs []string               `json:"addedCatalogIds"`
}

func toCatalogResponse(e *SkillCatalogEntry) catalogEntryResponse {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return catalogEntryResponse{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		Instructions: e.Instructions,
		Category:     e.Category,
		Tags:         tags,
		AuthorName:   e.AuthorName,
		Icon:         e.Icon,
		Featured:     e.Featured,
		AddCount:     e.AddCount,
	}
}

// list handles GET /v1/skill-catalog
func (h *catalogHandler) list(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	category := r.URL.Query().Get("category")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	entries, total, err := h.catalog.List(r.Context(), category, limit, offset)
	if err != nil {
		h.log.Error(r.Context(), "failed to list skill catalog", "error", err)
		http.Error(w, "Failed to list skill catalog", http.StatusInternalServerError)
		return
	}

	categories, err := h.catalog.Categories(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to fetch categories", "error", err)
		categories = []string{}
	}

	addedIDs, err := h.skillRepo.ListAddedCatalogIDs(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list added catalog IDs", "error", err)
		addedIDs = []string{}
	}

	resp := catalogListResponse{
		Entries:         make([]catalogEntryResponse, 0, len(entries)),
		Total:           total,
		Categories:      categories,
		AddedCatalogIDs: addedIDs,
	}
	for i := range entries {
		resp.Entries = append(resp.Entries, toCatalogResponse(&entries[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// search handles GET /v1/skill-catalog/search?q=...
func (h *catalogHandler) search(w http.ResponseWriter, r *http.Request) {
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

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	entries, total, err := h.catalog.Search(r.Context(), query, limit, offset)
	if err != nil {
		h.log.Error(r.Context(), "failed to search skill catalog", "error", err)
		http.Error(w, "Failed to search skill catalog", http.StatusInternalServerError)
		return
	}

	addedIDs, err := h.skillRepo.ListAddedCatalogIDs(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list added catalog IDs", "error", err)
		addedIDs = []string{}
	}

	resp := catalogListResponse{
		Entries:         make([]catalogEntryResponse, 0, len(entries)),
		Total:           total,
		AddedCatalogIDs: addedIDs,
	}
	for i := range entries {
		resp.Entries = append(resp.Entries, toCatalogResponse(&entries[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// addToMySkills handles POST /v1/skill-catalog/{id}/add
func (h *catalogHandler) addToMySkills(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	catalogID := r.PathValue("id")
	if catalogID == "" {
		http.Error(w, "Catalog entry ID is required", http.StatusBadRequest)
		return
	}

	entry, err := h.catalog.GetByID(r.Context(), catalogID)
	if err != nil {
		h.log.Error(r.Context(), "skill catalog entry not found", "id", catalogID, "error", err)
		http.Error(w, "Skill catalog entry not found", http.StatusNotFound)
		return
	}

	// Check for duplicate import
	exists, err := h.skillRepo.ExistsByCatalogID(r.Context(), user.ID, catalogID)
	if err != nil {
		h.log.Error(r.Context(), "failed to check duplicate", "error", err, "catalogID", catalogID)
		http.Error(w, "Failed to add skill", http.StatusInternalServerError)
		return
	}
	if exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":  "already_added",
			"detail": "You have already added this skill.",
		})
		return
	}

	// Check per-user skill limit
	count, err := h.skillRepo.CountByUser(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to count user skills", "error", err)
		http.Error(w, "Failed to add skill", http.StatusInternalServerError)
		return
	}
	if count >= maxSkillsPerUser {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{
			"error":  "limit_reached",
			"detail": "You have reached the maximum number of skills.",
		})
		return
	}

	skill := &UserSkill{
		UserID:          user.ID,
		Name:            entry.Name,
		Description:     entry.Description,
		Instructions:    entry.Instructions,
		Category:        entry.Category,
		Icon:            entry.Icon,
		IsCustom:        false,
		SourceCatalogID: catalogID,
	}

	if err := h.skillRepo.Create(r.Context(), skill); err != nil {
		h.log.Error(r.Context(), "failed to import catalog skill", "error", err, "catalogID", catalogID)
		http.Error(w, "Failed to add skill", http.StatusInternalServerError)
		return
	}

	_ = h.catalog.IncrementAddCount(r.Context(), catalogID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     skill.ID,
		"name":   skill.Name,
		"status": "added",
	})
}
