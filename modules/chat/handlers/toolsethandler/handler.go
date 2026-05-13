package toolsethandler

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/chat/handlers/common"
	"encoding/json"
	"net/http"
)

// Handler handles toolset-related HTTP endpoints.
type Handler struct {
	log                      *logger.Logger
	toolRegistry             *toolregistry.Registry
	requirementConfigService chatcontracts.ToolRequirementConfigService
}

// New creates a new toolset handler.
func New(
	log *logger.Logger,
	toolRegistry *toolregistry.Registry,
	requirementConfigService chatcontracts.ToolRequirementConfigService,
) *Handler {
	return &Handler{
		log:                      log,
		toolRegistry:             toolRegistry,
		requirementConfigService: requirementConfigService,
	}
}

// AgentToolSetResponse represents the API response format for agent tool sets.
type AgentToolSetResponse struct {
	ID                string                     `json:"id"`
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	Category          string                     `json:"category"`
	RequirementConfig map[string]any             `json:"requirement_config,omitempty"` // Per-toolset requirement config
	Tools             []common.ToolInSetResponse `json:"tools"`
}

// ListAgentToolSets returns a list of tool sets available to an agent,
// with requirement configs populated from the requirement-based system.
// GET /v1/agents/{agentId}/toolsets
func (h *Handler) ListAgentToolSets(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}
	allToolSets, err := h.toolRegistry.ListToolSets(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "Failed to list tool sets", "error", err)
		http.Error(w, "Failed to retrieve tool sets", http.StatusInternalServerError)
		return
	}
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	responseToolSets := make([]AgentToolSetResponse, 0, len(allToolSets))
	for _, toolSet := range allToolSets {
		ts, ok := h.buildAgentToolSetResponse(r, user.ID, agentID, toolSet)
		if !ok {
			continue
		}
		responseToolSets = append(responseToolSets, ts)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"toolsets": responseToolSets}); err != nil {
		h.log.Error(r.Context(), "failed to encode tool sets list", "error", err)
	}
}

// buildAgentToolSetResponse assembles one AgentToolSetResponse: loads
// the tools in the set, attaches per-tool and tool-set-level requirement
// configs. Returns ok=false when the tools list can't be loaded so the
// caller skips the entry.
func (h *Handler) buildAgentToolSetResponse(
	r *http.Request, userID, agentID string, toolSet toolregistry.ToolSet,
) (AgentToolSetResponse, bool) {
	toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSet.ID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSet.ID, "error", err)
		return AgentToolSetResponse{}, false
	}
	toolConfigs := h.loadToolConfigs(r, userID, agentID, toolSet.ID)
	toolResponses := make([]common.ToolInSetResponse, 0, len(toolsInSet))
	for _, tool := range toolsInSet {
		toolResponses = append(toolResponses, buildToolInSetResponse(tool, toolConfigs))
	}
	ts := AgentToolSetResponse{
		ID:          toolSet.ID,
		Name:        toolSet.Name,
		Description: toolSet.Description,
		Category:    toolSet.Category,
		Tools:       toolResponses,
	}
	if cfg := h.loadToolSetConfig(r, userID, agentID, toolSet.ID); cfg != nil {
		ts.RequirementConfig = cfg
	}
	return ts, true
}

// loadToolConfigs fetches per-tool requirement configs, nil when the
// service isn't wired.
func (h *Handler) loadToolConfigs(r *http.Request, userID, agentID, toolSetID string) map[string]string {
	if h.requirementConfigService == nil {
		return nil
	}
	configs, _ := h.requirementConfigService.ListToolRequirementConfigsJSON(r.Context(), userID, agentID, toolSetID)
	return configs
}

// loadToolSetConfig fetches the tool-set-level requirement config,
// parsing the JSON. Returns nil on missing / empty / malformed config.
func (h *Handler) loadToolSetConfig(r *http.Request, userID, agentID, toolSetID string) map[string]any {
	if h.requirementConfigService == nil {
		return nil
	}
	cfgJSON, _ := h.requirementConfigService.GetToolSetRequirementConfigJSON(r.Context(), userID, agentID, toolSetID)
	if cfgJSON == "" || cfgJSON == "{}" {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return nil
	}
	return cfg
}

// buildToolInSetResponse renders one tool's per-agent response with its
// requirement config attached when present.
func buildToolInSetResponse(tool toolregistry.Tool, configs map[string]string) common.ToolInSetResponse {
	tr := common.ToolInSetResponse{
		ID:          tool.ID,
		Name:        tool.Name,
		Description: tool.Description,
		Category:    tool.Category,
	}
	cfgJSON, ok := configs[tool.ID]
	if !ok || cfgJSON == "" || cfgJSON == "{}" {
		return tr
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(cfgJSON), &cfg); err != nil {
		return tr
	}
	tr.RequirementConfig = cfg
	return tr
}
