package templates

import (
	"aicompass/foundation/logger"
	"aicompass/modules/agentdefinition"
	"aicompass/shared"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// catalogHandler encapsulates HTTP handlers for the shared agent catalog.
type catalogHandler struct {
	log       *logger.Logger
	catalog   AgentCatalogRepository
	agentRepo agentdefinition.AgentDefinitionRepository
	importer  ComposedAgentImporter
}

func newCatalogHandler(
	log *logger.Logger,
	catalog AgentCatalogRepository,
	agentRepo agentdefinition.AgentDefinitionRepository,
	importer ComposedAgentImporter,
) *catalogHandler {
	return &catalogHandler{log: log, catalog: catalog, agentRepo: agentRepo, importer: importer}
}

// catalogEntryResponse represents a single catalog entry in API responses.
type catalogEntryResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	AuthorName  string   `json:"authorName"`
	Icon        string   `json:"icon"`
	Featured    bool     `json:"featured"`
	AddCount    int64    `json:"addCount"`
	IsComposed  bool     `json:"isComposed"`
}

// catalogListResponse is the response for listing/searching catalog entries.
type catalogListResponse struct {
	Entries    []catalogEntryResponse `json:"entries"`
	Total      int                    `json:"total"`
	Categories []string               `json:"categories,omitempty"`
}

func toCatalogResponse(e *agentdefinition.AgentCatalogEntry) catalogEntryResponse {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	return catalogEntryResponse{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Category:    e.Category,
		Tags:        tags,
		AuthorName:  e.AuthorName,
		Icon:        e.Icon,
		Featured:    e.Featured,
		AddCount:    e.AddCount,
		IsComposed:  e.IsWorkflow(),
	}
}

// list handles GET /v1/agent-catalog
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
		h.log.Error(r.Context(), "failed to list catalog", "error", err)
		http.Error(w, "Failed to list catalog", http.StatusInternalServerError)
		return
	}

	categories, err := h.catalog.Categories(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "failed to fetch categories", "error", err)
		categories = []string{}
	}

	resp := catalogListResponse{
		Entries:    make([]catalogEntryResponse, 0, len(entries)),
		Total:      total,
		Categories: categories,
	}
	for i := range entries {
		resp.Entries = append(resp.Entries, toCatalogResponse(&entries[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// search handles GET /v1/agent-catalog/search?q=...
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
		h.log.Error(r.Context(), "failed to search catalog", "error", err)
		http.Error(w, "Failed to search catalog", http.StatusInternalServerError)
		return
	}

	resp := catalogListResponse{
		Entries: make([]catalogEntryResponse, 0, len(entries)),
		Total:   total,
	}
	for i := range entries {
		resp.Entries = append(resp.Entries, toCatalogResponse(&entries[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// addToMyAgentsRequest represents the optional request body for adding a catalog entry.
type addToMyAgentsRequest struct {
	Name string `json:"name"`
}

// addToMyAgents handles POST /v1/agent-catalog/{id}/add
// In the unified model, both simple agents and composed agents are created as
// AgentDefinitions. Composed catalog entries create sub-agents as standalone
// reusable agents plus a composed agent with the graph referencing them.
func (h *catalogHandler) addToMyAgents(w http.ResponseWriter, r *http.Request) {
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

	// Parse optional request body for custom name
	var req addToMyAgentsRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	entry, err := h.catalog.GetByID(r.Context(), catalogID)
	if err != nil {
		h.log.Error(r.Context(), "catalog entry not found", "id", catalogID, "error", err)
		http.Error(w, "Catalog entry not found", http.StatusNotFound)
		return
	}

	// Use custom name if provided, otherwise fall back to template name
	agentName := entry.Name
	if req.Name != "" {
		agentName = req.Name
	}

	// ── Composed catalog entry: create sub-agents + composed agent ──
	if entry.IsWorkflow() {
		agentID, forkErr := h.forkComposedCatalogEntry(r.Context(), user.ID, catalogID, entry, agentName)
		if forkErr != nil {
			h.log.Error(r.Context(), "failed to fork composed catalog entry", "error", forkErr, "catalogID", catalogID)
			http.Error(w, "Failed to add agent", http.StatusInternalServerError)
			return
		}

		_ = h.catalog.IncrementAddCount(r.Context(), catalogID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"id":     agentID,
			"name":   agentName,
			"status": "added",
		})
		return
	}

	// ── Simple agent catalog entry ──
	agent := &agentdefinition.AgentDefinition{
		UserID:          user.ID,
		Name:            agentName,
		Description:     entry.Description,
		SystemPrompt:    entry.SystemPrompt,
		Icon:            entry.Icon,
		SourceCatalogID: catalogID,
	}

	if err := h.agentRepo.Create(r.Context(), agent); err != nil {
		h.log.Error(r.Context(), "failed to fork catalog agent", "error", err, "catalogID", catalogID)
		http.Error(w, "Failed to add agent", http.StatusInternalServerError)
		return
	}

	_ = h.catalog.IncrementAddCount(r.Context(), catalogID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"id":     agent.ID,
		"name":   agent.Name,
		"status": "added",
	})
}

// forkComposedCatalogEntry creates standalone sub-agents and a composed parent agent
// from a catalog entry with a workflow graph.
func (h *catalogHandler) forkComposedCatalogEntry(
	ctx context.Context,
	userID, catalogID string,
	entry *agentdefinition.AgentCatalogEntry,
	customName string,
) (string, error) {
	return h.importer.ImportComposedFromCatalogEntry(ctx, userID, entry, catalogID, customName)
}
