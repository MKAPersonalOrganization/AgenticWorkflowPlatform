// Package agentcatalog provides HTTP handlers for the shared agent store catalog.
package agentcatalog

import (
	"encoding/json"
	"net/http"
	"strconv"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
)

// Handler encapsulates HTTP handlers for the agent catalog.
type Handler struct {
	log       *logger.Logger
	catalog   domain.AgentCatalogRepository
	agentRepo domain.AgentDefinitionRepository
}

// NewHandler creates a new agent catalog HTTP handler.
func NewHandler(log *logger.Logger, catalog domain.AgentCatalogRepository, agentRepo domain.AgentDefinitionRepository) *Handler {
	return &Handler{log: log, catalog: catalog, agentRepo: agentRepo}
}

// CatalogEntryResponse represents a single catalog entry in API responses.
type CatalogEntryResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	AuthorName  string   `json:"authorName"`
	IconLabel   string   `json:"iconLabel"`
	Featured    bool     `json:"featured"`
	AddCount    int64    `json:"addCount"`
}

// CatalogListResponse is the response for listing/searching catalog entries.
type CatalogListResponse struct {
	Entries         []CatalogEntryResponse `json:"entries"`
	Total           int                    `json:"total"`
	Categories      []string               `json:"categories,omitempty"`
	AddedCatalogIDs []string               `json:"addedCatalogIds"`
}

func toCatalogResponse(e *domain.AgentCatalogEntry) CatalogEntryResponse {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return CatalogEntryResponse{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Category:    e.Category,
		Tags:        tags,
		AuthorName:  e.AuthorName,
		IconLabel:   e.IconLabel,
		Featured:    e.Featured,
		AddCount:    e.AddCount,
	}
}

// List handles GET /v1/agent-catalog
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
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
		h.log.Error(r.Context(), "failed to list catalog", "error", err)
		http.Error(w, "Failed to list catalog", http.StatusInternalServerError)
		return
	}

	// Fetch categories for the filter UI
	categories, err := h.catalog.Categories(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to fetch categories", "error", err)
		categories = []string{}
	}

	// Fetch catalog IDs the user already added (single indexed query)
	addedIDs, err := h.agentRepo.ListAddedCatalogIDs(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list added catalog IDs", "error", err)
		addedIDs = []string{}
	}

	resp := CatalogListResponse{
		Entries:         make([]CatalogEntryResponse, 0, len(entries)),
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

// Search handles GET /v1/agent-catalog/search?q=...
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		h.List(w, r)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	entries, total, err := h.catalog.Search(r.Context(), query, limit, offset)
	if err != nil {
		h.log.Error(r.Context(), "failed to search catalog", "error", err)
		http.Error(w, "Failed to search catalog", http.StatusInternalServerError)
		return
	}

	// Fetch catalog IDs the user already added
	addedIDs, err := h.agentRepo.ListAddedCatalogIDs(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list added catalog IDs", "error", err)
		addedIDs = []string{}
	}

	resp := CatalogListResponse{
		Entries:         make([]CatalogEntryResponse, 0, len(entries)),
		Total:           total,
		AddedCatalogIDs: addedIDs,
	}
	for i := range entries {
		resp.Entries = append(resp.Entries, toCatalogResponse(&entries[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// AddToMyAgents handles POST /v1/agent-catalog/{id}/add
// Forks a catalog entry into the user's personal agent definitions.
func (h *Handler) AddToMyAgents(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	catalogID := r.PathValue("id")
	if catalogID == "" {
		http.Error(w, "Catalog entry ID is required", http.StatusBadRequest)
		return
	}

	// Fetch the catalog entry
	entry, err := h.catalog.GetByID(r.Context(), catalogID)
	if err != nil {
		h.log.Error(r.Context(), "catalog entry not found", "id", catalogID, "error", err)
		http.Error(w, "Catalog entry not found", http.StatusNotFound)
		return
	}

	// Duplicate guard — check if the user already forked this catalog agent
	exists, err := h.agentRepo.ExistsByCatalogID(r.Context(), user.ID, catalogID)
	if err != nil {
		h.log.Error(r.Context(), "failed to check duplicate", "error", err, "catalogID", catalogID)
		http.Error(w, "Failed to add agent", http.StatusInternalServerError)
		return
	}
	if exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error":  "already_added",
			"detail": "You have already added this agent.",
		})
		return
	}

	// Create a personal copy
	agent := &domain.AgentDefinition{
		UserID:          user.ID,
		Name:            entry.Name,
		Description:     entry.Description,
		SystemPrompt:    entry.SystemPrompt,
		SourceCatalogID: catalogID,
		MaxIterations:   20,
		Temperature:     0.7,
	}

	if err := h.agentRepo.Create(r.Context(), agent); err != nil {
		h.log.Error(r.Context(), "failed to fork catalog agent", "error", err, "catalogID", catalogID)
		http.Error(w, "Failed to add agent", http.StatusInternalServerError)
		return
	}

	// Increment popularity counter (best-effort)
	_ = h.catalog.IncrementAddCount(r.Context(), catalogID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     agent.ID,
		"name":   agent.Name,
		"status": "added",
	})
}
