// Package agentstore provides HTTP handlers for user-created agent definitions.
package agentstore

import (
	"encoding/json"
	"net/http"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/services"
	"strings"

	agentStoreService "school-with-ai/services/agentstore"
)

// Handler encapsulates HTTP handlers for agent CRUD operations.
type Handler struct {
	log         *logger.Logger
	service     *agentStoreService.Service
	authService services.AuthService
}

// NewHandler creates a new agent store HTTP handler.
func NewHandler(log *logger.Logger, service *agentStoreService.Service, authService services.AuthService) *Handler {
	return &Handler{
		log:         log,
		service:     service,
		authService: authService,
	}
}

// CreateRequest represents the request body for creating an agent.
type CreateRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
}

// UpdateRequest represents the request body for updating an agent.
type UpdateRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
}

// AgentResponse represents a single agent in API responses.
type AgentResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	SystemPrompt    string `json:"systemPrompt"`
	Icon            string `json:"icon"`
	SourceCatalogID string `json:"sourceCatalogId,omitempty"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// ListResponse represents the response for listing agents.
type ListResponse struct {
	Agents []AgentResponse `json:"agents"`
}

// defaultAgentIconSVG is the fallback icon (bot) for agents without a valid SVG icon.
var defaultAgentIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>`

// agentIconOrDefault returns the icon if it's a valid SVG, otherwise the default bot icon.
func agentIconOrDefault(icon string) string {
	if strings.HasPrefix(icon, "<svg") {
		return icon
	}
	return defaultAgentIconSVG
}

func toAgentResponse(a *domain.AgentDefinition) AgentResponse {
	return AgentResponse{
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

// Create handles POST /v1/agent-definitions
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateRequest
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
	}

	if err := h.service.Create(r.Context(), agent); err != nil {
		h.log.Error(r.Context(), "failed to create agent", "error", err)
		http.Error(w, "Failed to create agent", http.StatusInternalServerError)
		return
	}

	resp := toAgentResponse(agent)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// Update handles PUT /v1/agent-definitions/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Agent name is required", http.StatusBadRequest)
		return
	}

	agent := &domain.AgentDefinition{
		ID:           agentID,
		UserID:       user.ID,
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
	}

	if err := h.service.Update(r.Context(), agent); err != nil {
		h.log.Error(r.Context(), "failed to update agent", "error", err)
		http.Error(w, "Failed to update agent", http.StatusInternalServerError)
		return
	}

	// Fetch the updated agent to return complete data
	updated, err := h.service.GetByID(r.Context(), agentID, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to fetch updated agent", "error", err)
		http.Error(w, "Failed to fetch updated agent", http.StatusInternalServerError)
		return
	}

	resp := toAgentResponse(updated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Delete handles DELETE /v1/agent-definitions/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	if err := h.service.Delete(r.Context(), agentID, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to delete agent", "error", err)
		http.Error(w, "Failed to delete agent", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// List handles GET /v1/agent-definitions
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agents, err := h.service.ListByUser(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list agents", "error", err)
		http.Error(w, "Failed to list agents", http.StatusInternalServerError)
		return
	}

	resp := ListResponse{
		Agents: make([]AgentResponse, 0, len(agents)),
	}
	for i := range agents {
		resp.Agents = append(resp.Agents, toAgentResponse(&agents[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Get handles GET /v1/agent-definitions/{id}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	agent, err := h.service.GetByID(r.Context(), agentID, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get agent", "error", err)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	resp := toAgentResponse(agent)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Search handles GET /v1/agent-definitions/search?q=...
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		// Return all agents if no search query
		h.List(w, r)
		return
	}

	agents, err := h.service.Search(r.Context(), user.ID, query)
	if err != nil {
		h.log.Error(r.Context(), "failed to search agents", "error", err)
		http.Error(w, "Failed to search agents", http.StatusInternalServerError)
		return
	}

	resp := ListResponse{
		Agents: make([]AgentResponse, 0, len(agents)),
	}
	for i := range agents {
		resp.Agents = append(resp.Agents, toAgentResponse(&agents[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
