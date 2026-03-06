package agentstore

import (
"encoding/json"
"net/http"
"strings"

"school-with-ai/domain"
"school-with-ai/foundation/logger"
)

// storeHandler encapsulates HTTP handlers for agent CRUD operations.
type storeHandler struct {
log *logger.Logger
svc *service
}

// newStoreHandler creates a new agent store HTTP handler.
func newStoreHandler(log *logger.Logger, svc *service) *storeHandler {
return &storeHandler{log: log, svc: svc}
}

// createRequest represents the request body for creating an agent.
type createRequest struct {
Name         string `json:"name"`
Description  string `json:"description"`
SystemPrompt string `json:"systemPrompt"`
Icon         string `json:"icon"`
}

// updateRequest represents the request body for updating an agent.
type updateRequest struct {
Name         string `json:"name"`
Description  string `json:"description"`
SystemPrompt string `json:"systemPrompt"`
Icon         string `json:"icon"`
}

// agentResponse represents a single agent in API responses.
type agentResponse struct {
ID              string `json:"id"`
Name            string `json:"name"`
Description     string `json:"description"`
SystemPrompt    string `json:"systemPrompt"`
Icon            string `json:"icon"`
SourceCatalogID string `json:"sourceCatalogId,omitempty"`
CreatedAt       string `json:"createdAt"`
UpdatedAt       string `json:"updatedAt"`
}

// listResponse represents the response for listing agents.
type listResponse struct {
Agents []agentResponse `json:"agents"`
}

// defaultAgentIconSVG is the fallback icon (compass) for agents without a valid SVG icon.
var defaultAgentIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="m16.24 7.76-1.804 5.411a2 2 0 0 1-1.265 1.265L7.76 16.24l1.804-5.411a2 2 0 0 1 1.265-1.265z" /></svg>`

// agentIconOrDefault returns the icon if it's a valid SVG, otherwise the default compass icon.
func agentIconOrDefault(icon string) string {
if strings.HasPrefix(icon, "<svg") {
return icon
}
return defaultAgentIconSVG
}

func toAgentResponse(a *domain.AgentDefinition) agentResponse {
return agentResponse{
ID:              a.ID,
Name:            a.Name,
Description:     a.Description,
SystemPrompt:    a.SystemPrompt,
Icon:            agentIconOrDefault(a.Icon),
SourceCatalogID: a.SourceCatalogID,
CreatedAt:       a.CreatedAt.Format("2006-01-02T15:04:05Z"),
UpdatedAt:       a.UpdatedAt.Format("2006-01-02T15:04:05Z"),
}
}

// create handles POST /v1/agent-definitions
func (h *storeHandler) create(w http.ResponseWriter, r *http.Request) {
user := domain.GetUserFromContext(r.Context())
if user == nil {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}

var req createRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "Invalid request body", http.StatusBadRequest)
return
}

if req.Name == "" {
http.Error(w, "Agent name is required", http.StatusBadRequest)
return
}

agent := &domain.AgentDefinition{
UserID:       user.ID,
Name:         req.Name,
Description:  req.Description,
SystemPrompt: req.SystemPrompt,
Icon:         agentIconOrDefault(req.Icon),
}

if err := h.svc.create(r.Context(), agent); err != nil {
h.log.Error(r.Context(), "failed to create agent", "error", err)
http.Error(w, "Failed to create agent", http.StatusInternalServerError)
return
}

resp := toAgentResponse(agent)
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusCreated)
json.NewEncoder(w).Encode(resp)
}

// update handles PUT /v1/agent-definitions/{id}
func (h *storeHandler) update(w http.ResponseWriter, r *http.Request) {
user := domain.GetUserFromContext(r.Context())
if user == nil {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}

agentID := r.PathValue("id")
if agentID == "" {
http.Error(w, "Agent ID is required", http.StatusBadRequest)
return
}

var req updateRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "Invalid request body", http.StatusBadRequest)
return
}

if req.Name == "" {
http.Error(w, "Agent name is required", http.StatusBadRequest)
return
}

// Fetch the existing agent to preserve the icon if the request doesn't provide one.
existing, err := h.svc.getByID(r.Context(), agentID, user.ID)
if err != nil {
h.log.Error(r.Context(), "agent not found", "error", err)
http.Error(w, "Agent not found", http.StatusNotFound)
return
}

iconValue := existing.Icon
if req.Icon != "" {
iconValue = req.Icon
}

agent := &domain.AgentDefinition{
ID:           agentID,
UserID:       user.ID,
Name:         req.Name,
Description:  req.Description,
SystemPrompt: req.SystemPrompt,
Icon:         iconValue,
}

if err := h.svc.update(r.Context(), agent); err != nil {
h.log.Error(r.Context(), "failed to update agent", "error", err)
http.Error(w, "Failed to update agent", http.StatusInternalServerError)
return
}

// Fetch the updated agent to return complete data
updated, err := h.svc.getByID(r.Context(), agentID, user.ID)
if err != nil {
h.log.Error(r.Context(), "failed to fetch updated agent", "error", err)
http.Error(w, "Failed to fetch updated agent", http.StatusInternalServerError)
return
}

resp := toAgentResponse(updated)
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}

// delete handles DELETE /v1/agent-definitions/{id}
func (h *storeHandler) delete(w http.ResponseWriter, r *http.Request) {
user := domain.GetUserFromContext(r.Context())
if user == nil {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}

agentID := r.PathValue("id")
if agentID == "" {
http.Error(w, "Agent ID is required", http.StatusBadRequest)
return
}

if err := h.svc.delete(r.Context(), agentID, user.ID); err != nil {
h.log.Error(r.Context(), "failed to delete agent", "error", err)
http.Error(w, "Failed to delete agent", http.StatusInternalServerError)
return
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// list handles GET /v1/agent-definitions
func (h *storeHandler) list(w http.ResponseWriter, r *http.Request) {
user := domain.GetUserFromContext(r.Context())
if user == nil {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}

agents, err := h.svc.listByUser(r.Context(), user.ID)
if err != nil {
h.log.Error(r.Context(), "failed to list agents", "error", err)
http.Error(w, "Failed to list agents", http.StatusInternalServerError)
return
}

resp := listResponse{
Agents: make([]agentResponse, 0, len(agents)),
}
for i := range agents {
resp.Agents = append(resp.Agents, toAgentResponse(&agents[i]))
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}

// get handles GET /v1/agent-definitions/{id}
func (h *storeHandler) get(w http.ResponseWriter, r *http.Request) {
user := domain.GetUserFromContext(r.Context())
if user == nil {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}

agentID := r.PathValue("id")
if agentID == "" {
http.Error(w, "Agent ID is required", http.StatusBadRequest)
return
}

agent, err := h.svc.getByID(r.Context(), agentID, user.ID)
if err != nil {
h.log.Error(r.Context(), "failed to get agent", "error", err)
http.Error(w, "Agent not found", http.StatusNotFound)
return
}

resp := toAgentResponse(agent)
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}

// search handles GET /v1/agent-definitions/search?q=...
func (h *storeHandler) search(w http.ResponseWriter, r *http.Request) {
user := domain.GetUserFromContext(r.Context())
if user == nil {
http.Error(w, "Unauthorized", http.StatusUnauthorized)
return
}

query := r.URL.Query().Get("q")
if query == "" {
// Return all agents if no search query
h.list(w, r)
return
}

agents, err := h.svc.search(r.Context(), user.ID, query)
if err != nil {
h.log.Error(r.Context(), "failed to search agents", "error", err)
http.Error(w, "Failed to search agents", http.StatusInternalServerError)
return
}

resp := listResponse{
Agents: make([]agentResponse, 0, len(agents)),
}
for i := range agents {
resp.Agents = append(resp.Agents, toAgentResponse(&agents[i]))
}

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(resp)
}
