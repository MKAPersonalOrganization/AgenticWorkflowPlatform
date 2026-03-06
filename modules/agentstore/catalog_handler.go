package agentstore

import (
"encoding/json"
"net/http"
"strconv"

"school-with-ai/domain"
"school-with-ai/foundation/logger"
)

// catalogHandler encapsulates HTTP handlers for the shared agent catalog.
type catalogHandler struct {
log       *logger.Logger
catalog   domain.AgentCatalogRepository
agentRepo domain.AgentDefinitionRepository
}

func newCatalogHandler(log *logger.Logger, catalog domain.AgentCatalogRepository, agentRepo domain.AgentDefinitionRepository) *catalogHandler {
return &catalogHandler{log: log, catalog: catalog, agentRepo: agentRepo}
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
}

// catalogListResponse is the response for listing/searching catalog entries.
type catalogListResponse struct {
Entries         []catalogEntryResponse `json:"entries"`
Total           int                    `json:"total"`
Categories      []string               `json:"categories,omitempty"`
AddedCatalogIDs []string               `json:"addedCatalogIds"`
}

func toCatalogResponse(e *domain.AgentCatalogEntry) catalogEntryResponse {
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
}
}

// list handles GET /v1/agent-catalog
func (h *catalogHandler) list(w http.ResponseWriter, r *http.Request) {
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

categories, err := h.catalog.Categories(r.Context())
if err != nil {
h.log.Error(r.Context(), "failed to fetch categories", "error", err)
categories = []string{}
}

addedIDs, err := h.agentRepo.ListAddedCatalogIDs(r.Context(), user.ID)
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

// search handles GET /v1/agent-catalog/search?q=...
func (h *catalogHandler) search(w http.ResponseWriter, r *http.Request) {
user := domain.GetUserFromContext(r.Context())
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

addedIDs, err := h.agentRepo.ListAddedCatalogIDs(r.Context(), user.ID)
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

// addToMyAgents handles POST /v1/agent-catalog/{id}/add
func (h *catalogHandler) addToMyAgents(w http.ResponseWriter, r *http.Request) {
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

entry, err := h.catalog.GetByID(r.Context(), catalogID)
if err != nil {
h.log.Error(r.Context(), "catalog entry not found", "id", catalogID, "error", err)
http.Error(w, "Catalog entry not found", http.StatusNotFound)
return
}

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

agent := &domain.AgentDefinition{
UserID:          user.ID,
Name:            entry.Name,
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
