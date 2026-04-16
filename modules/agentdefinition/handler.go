package agentdefinition

import (
	"aicompass/foundation/logger"
	"aicompass/foundation/auth"
	"encoding/json"
	"net/http"
	"strings"
)

// storeHandler encapsulates HTTP handlers for agent CRUD and sharing operations.
type storeHandler struct {
	log *logger.Logger
	svc *Service
}

func newStoreHandler(log *logger.Logger, svc *Service) *storeHandler {
	return &storeHandler{log: log, svc: svc}
}

// ── Request / Response Types ──
// These use camelCase JSON tags for the frontend.

// agentModelConfigRequest is the JSON shape for per-agent model settings in
// create/update requests. All fields are optional; omitted fields are treated
// as "use global default".
type agentModelConfigRequest struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *float32 `json:"topK,omitempty"`
	MaxOutputTokens *int32   `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// agentModelOverrideRequest is the JSON shape for selecting a specific
// provider + model for an agent, overriding the user's global AI settings.
type agentModelOverrideRequest struct {
	Provider  string `json:"provider"`
	ModelName string `json:"modelName"`
}

type createRequest struct {
	Name                     string                     `json:"name"`
	Description              string                     `json:"description"`
	SystemPrompt             string                     `json:"systemPrompt"`
	Icon                     string                     `json:"icon"`
	Graph                    *CompositionNode           `json:"graph,omitempty"`
	SubAgents                []*SubAgentRef           `json:"subAgents,omitempty"`
	DisallowTransferToParent bool                       `json:"disallowTransferToParent"`
	DisallowTransferToPeers  bool                       `json:"disallowTransferToPeers"`
	OutputKey                string                     `json:"outputKey"`
	IncludeContents          string                     `json:"includeContents"`
	GlobalInstruction        string                     `json:"globalInstruction"`
	InputSchema              string                     `json:"inputSchema"`
	OutputSchema             string                     `json:"outputSchema"`
	ModelConfig              *agentModelConfigRequest   `json:"modelConfig,omitempty"`
	ModelOverride            *agentModelOverrideRequest `json:"modelOverride,omitempty"`
}

type updateRequest struct {
	Name                     string                     `json:"name"`
	Description              string                     `json:"description"`
	SystemPrompt             string                     `json:"systemPrompt"`
	Icon                     string                     `json:"icon"`
	Graph                    *CompositionNode           `json:"graph"`
	SubAgents                []*SubAgentRef           `json:"subAgents,omitempty"`
	DisallowTransferToParent bool                       `json:"disallowTransferToParent"`
	DisallowTransferToPeers  bool                       `json:"disallowTransferToPeers"`
	OutputKey                string                     `json:"outputKey"`
	IncludeContents          string                     `json:"includeContents"`
	GlobalInstruction        string                     `json:"globalInstruction"`
	InputSchema              string                     `json:"inputSchema"`
	OutputSchema             string                     `json:"outputSchema"`
	ModelConfig              *agentModelConfigRequest   `json:"modelConfig,omitempty"`
	ModelOverride            *agentModelOverrideRequest `json:"modelOverride,omitempty"`
}

type agentResponse struct {
	ID                       string                   `json:"id"`
	Name                     string                   `json:"name"`
	Description              string                   `json:"description"`
	SystemPrompt             string                   `json:"systemPrompt"`
	Icon                     string                   `json:"icon"`
	SourceCatalogID          string                   `json:"sourceCatalogId,omitempty"`
	Graph                    *CompositionNode         `json:"graph,omitempty"`
	IsComposed               bool                     `json:"isComposed"`
	IsCoordinator            bool                     `json:"isCoordinator"`
	SubAgents                []*SubAgentRef           `json:"subAgents,omitempty"`
	DisallowTransferToParent bool                     `json:"disallowTransferToParent"`
	DisallowTransferToPeers  bool                     `json:"disallowTransferToPeers"`
	OutputKey                string                   `json:"outputKey,omitempty"`
	IncludeContents          string                   `json:"includeContents,omitempty"`
	GlobalInstruction        string                   `json:"globalInstruction,omitempty"`
	InputSchema              string                   `json:"inputSchema,omitempty"`
	OutputSchema             string                   `json:"outputSchema,omitempty"`
	ModelConfig              *AgentModelConfig        `json:"modelConfig,omitempty"`
	ModelOverride            *AgentModelOverride      `json:"modelOverride,omitempty"`
	ShareToken               string                   `json:"shareToken,omitempty"`
	Visibility               string                   `json:"visibility,omitempty"`
	CreatedAt                string                   `json:"createdAt"`
	UpdatedAt                string                   `json:"updatedAt"`
}

type listResponse struct {
	Agents []agentResponse `json:"agents"`
}

// defaultAgentIconSVG is the fallback icon (compass) for agents without a valid SVG icon.
var defaultAgentIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="m16.24 7.76-1.804 5.411a2 2 0 0 1-1.265 1.265L7.76 16.24l1.804-5.411a2 2 0 0 1 1.265-1.265z" /></svg>`

func agentIconOrDefault(icon string) string {
	if strings.HasPrefix(icon, "<svg") {
		return icon
	}
	return defaultAgentIconSVG
}

// toModelConfig converts a request model config to the domain type.
// Returns nil when the request carries no model config.
func toModelConfig(req *agentModelConfigRequest) *AgentModelConfig {
	if req == nil {
		return nil
	}
	return &AgentModelConfig{
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		TopK:            req.TopK,
		MaxOutputTokens: req.MaxOutputTokens,
		StopSequences:   req.StopSequences,
	}
}

// toModelOverride converts a request model override to the domain type.
// Returns nil when both provider and modelName are empty (no override requested).
func toModelOverride(req *agentModelOverrideRequest) *AgentModelOverride {
	if req == nil || (req.Provider == "" && req.ModelName == "") {
		return nil
	}
	return &AgentModelOverride{
		Provider:  req.Provider,
		ModelName: req.ModelName,
	}
}

func toAgentResponse(a *AgentDefinition) agentResponse {
	return agentResponse{
		ID:                       a.ID,
		Name:                     a.Name,
		Description:              a.Description,
		SystemPrompt:             a.SystemPrompt,
		Icon:                     agentIconOrDefault(a.Icon),
		SourceCatalogID:          a.SourceCatalogID,
		Graph:                    a.Graph,
		IsComposed:               a.IsComposed(),
		IsCoordinator:            a.IsCoordinator(),
		SubAgents:                a.SubAgents,
		DisallowTransferToParent: a.DisallowTransferToParent,
		DisallowTransferToPeers:  a.DisallowTransferToPeers,
		OutputKey:                a.OutputKey,
		IncludeContents:          a.IncludeContents,
		GlobalInstruction:        a.GlobalInstruction,
		InputSchema:              a.InputSchemaJSON,
		OutputSchema:             a.OutputSchemaJSON,
		ModelConfig:              a.ModelConfig,
		ModelOverride:            a.ModelOverride,
		ShareToken:               a.ShareToken,
		Visibility:               a.Visibility,
		CreatedAt:                a.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:                a.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// ── CRUD Handlers ──

// create handles POST /v1/agent-definitions
func (h *storeHandler) create(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
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

	agent := &AgentDefinition{
		UserID:                   user.ID,
		Name:                     req.Name,
		Description:              req.Description,
		SystemPrompt:             req.SystemPrompt,
		Icon:                     agentIconOrDefault(req.Icon),
		Graph:                    req.Graph,
		SubAgents:                req.SubAgents,
		DisallowTransferToParent: req.DisallowTransferToParent,
		DisallowTransferToPeers:  req.DisallowTransferToPeers,
		OutputKey:                req.OutputKey,
		IncludeContents:          req.IncludeContents,
		GlobalInstruction:        req.GlobalInstruction,
		InputSchemaJSON:          req.InputSchema,
		OutputSchemaJSON:         req.OutputSchema,
		ModelConfig:              toModelConfig(req.ModelConfig),
		ModelOverride:            toModelOverride(req.ModelOverride),
	}

	if err := h.svc.create(r.Context(), agent); err != nil {
		h.log.Error(r.Context(), "failed to create agent", "error", err)
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := toAgentResponse(agent)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// importYAML handles POST /v1/agent-definitions/import-yaml
func (h *storeHandler) importYAML(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		YAML string `json:"yaml"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	urlTrim := strings.TrimSpace(req.URL)
	yamlTrim := strings.TrimSpace(req.YAML)
	var raw []byte
	if urlTrim != "" && yamlTrim != "" {
		writeJSONError(w, "Provide either url or yaml, not both", http.StatusBadRequest)
		return
	}
	if urlTrim != "" {
		var err error
		raw, err = fetchYAMLFromURL(r.Context(), urlTrim)
		if err != nil {
			h.log.Error(r.Context(), "import yaml fetch url", "error", err)
			writeJSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		raw = []byte(req.YAML)
	}
	if strings.TrimSpace(string(raw)) == "" {
		writeJSONError(w, "YAML body is empty", http.StatusBadRequest)
		return
	}

	id, isComposed, err := h.svc.ImportFromCatalogYAML(r.Context(), user.ID, raw)
	if err != nil {
		h.log.Error(r.Context(), "import yaml", "error", err)
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":         id,
		"isComposed": isComposed,
	})
}

// update handles PUT /v1/agent-definitions/{id}
func (h *storeHandler) update(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
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

	agent := &AgentDefinition{
		ID:                       agentID,
		UserID:                   user.ID,
		Name:                     req.Name,
		Description:              req.Description,
		SystemPrompt:             req.SystemPrompt,
		Icon:                     iconValue,
		Graph:                    req.Graph,
		SubAgents:                req.SubAgents,
		DisallowTransferToParent: req.DisallowTransferToParent,
		DisallowTransferToPeers:  req.DisallowTransferToPeers,
		OutputKey:                req.OutputKey,
		IncludeContents:          req.IncludeContents,
		GlobalInstruction:        req.GlobalInstruction,
		InputSchemaJSON:          req.InputSchema,
		OutputSchemaJSON:         req.OutputSchema,
		ModelConfig:              toModelConfig(req.ModelConfig),
		ModelOverride:            toModelOverride(req.ModelOverride),
	}

	if err := h.svc.update(r.Context(), agent); err != nil {
		h.log.Error(r.Context(), "failed to update agent", "error", err)
		writeJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	user := auth.GetUserFromContext(r.Context())
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

// deleteAll handles DELETE /v1/agent-definitions
func (h *storeHandler) deleteAll(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agents, err := h.svc.listByUser(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list agents for bulk delete", "error", err)
		http.Error(w, "Failed to list agents", http.StatusInternalServerError)
		return
	}

	deleted := 0
	failed := 0
	for i := range agents {
		if err := h.svc.delete(r.Context(), agents[i].ID, user.ID); err != nil {
			h.log.Error(r.Context(), "failed to delete agent during bulk delete",
				"agentID", agents[i].ID, "error", err)
			failed++
		} else {
			deleted++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "completed",
		"deleted":  deleted,
		"failed":   failed,
		"total":    len(agents),
	})
}

// list handles GET /v1/agent-definitions
func (h *storeHandler) list(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
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
	user := auth.GetUserFromContext(r.Context())
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
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
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
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// consumers handles GET /v1/agent-definitions/{id}/consumers
func (h *storeHandler) consumers(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	consumers, err := h.svc.GetConsumers(r.Context(), user.ID, agentID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get consumers", "error", err)
		http.Error(w, "Failed to get consumers", http.StatusInternalServerError)
		return
	}

	resp := listResponse{
		Agents: make([]agentResponse, 0, len(consumers)),
	}
	for i := range consumers {
		resp.Agents = append(resp.Agents, toAgentResponse(&consumers[i]))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ── Sharing Handlers ──

// shareAgent handles POST /v1/agent-definitions/{id}/share
func (h *storeHandler) shareAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	token, err := h.svc.share(r.Context(), agentID, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to share agent", "error", err)
		writeJSONError(w, "Failed to share agent", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"shareToken": token,
		"shareUrl":   "/shared/agents/" + token,
	})
}

// unshareAgent handles DELETE /v1/agent-definitions/{id}/share
func (h *storeHandler) unshareAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.PathValue("id")
	if agentID == "" {
		http.Error(w, "Agent ID is required", http.StatusBadRequest)
		return
	}

	if err := h.svc.unshare(r.Context(), agentID, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to unshare agent", "error", err)
		writeJSONError(w, "Failed to unshare agent", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "unshared"})
}

// getSharedAgent handles GET /v1/shared/agents/{token}
func (h *storeHandler) getSharedAgent(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "Share token is required", http.StatusBadRequest)
		return
	}

	agent, err := h.svc.getShared(r.Context(), token)
	if err != nil {
		http.Error(w, "Shared agent not found", http.StatusNotFound)
		return
	}

	// Return a sanitized response (no user ID, no internal fields).
	resp := agentResponse{
		ID:                       agent.ID,
		Name:                     agent.Name,
		Description:              agent.Description,
		SystemPrompt:             agent.SystemPrompt,
		Icon:                     agentIconOrDefault(agent.Icon),
		Graph:                    agent.Graph,
		IsComposed:               agent.IsComposed(),
		DisallowTransferToParent: agent.DisallowTransferToParent,
		DisallowTransferToPeers:  agent.DisallowTransferToPeers,
		OutputKey:                agent.OutputKey,
		IncludeContents:          agent.IncludeContents,
		GlobalInstruction:        agent.GlobalInstruction,
		CreatedAt:                agent.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:                agent.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// forkSharedAgent handles POST /v1/shared/agents/{token}/fork
func (h *storeHandler) forkSharedAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "Share token is required", http.StatusBadRequest)
		return
	}

	forked, err := h.svc.forkShared(r.Context(), token, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to fork shared agent", "error", err)
		writeJSONError(w, "Failed to fork agent", http.StatusInternalServerError)
		return
	}

	resp := toAgentResponse(forked)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
