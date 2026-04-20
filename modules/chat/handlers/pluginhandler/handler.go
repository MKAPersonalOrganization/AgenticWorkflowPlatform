package pluginhandler

import (
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/chat/handlers/common"
	"aicompass/foundation/auth"
	"encoding/json"
	"net/http"
)

// Handler handles plugin-related HTTP endpoints.
type Handler struct {
	log                      *logger.Logger
	toolRegistry             *toolregistry.Registry
	requirementConfigService chatcontracts.ToolRequirementConfigService
}

// New creates a new plugin handler.
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

// PluginResponse represents a plugin (tool set) in the simplified API.
type PluginResponse struct {
	ID                string                     `json:"id"`
	PluginID          string                     `json:"plugin_id"` // Parent plugin provider ID for settings lookup
	Name              string                     `json:"name"`
	Description       string                     `json:"description"`
	Category          string                     `json:"category"`
	Icon              string                     `json:"icon"`
	RequirementConfig map[string]any             `json:"requirement_config,omitempty"` // Per-plugin requirement config
	Tools             []common.ToolInSetResponse `json:"tools"`
}

// ListPlugins returns all available plugins with their requirement configs.
// GET /v1/plugins (global — no per-agent state)
// GET /v1/plugins?agent_id=xxx (per-agent requirement configs for agent config)
func (h *Handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	agentID := r.URL.Query().Get("agent_id")

	allToolSets, err := h.toolRegistry.ListToolSets(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "Failed to list tool sets", "error", err)
		http.Error(w, "Failed to retrieve plugins", http.StatusInternalServerError)
		return
	}

	plugins := make([]PluginResponse, 0, len(allToolSets))
	for _, toolSet := range allToolSets {
		pr, ok := h.buildPluginResponse(r, user.ID, agentID, toolSet)
		if !ok {
			continue
		}
		plugins = append(plugins, pr)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"plugins": plugins}); err != nil {
		h.log.Error(r.Context(), "failed to encode plugins list", "error", err)
	}
}

// buildPluginResponse assembles one PluginResponse for the ListPlugins
// result: loads the tools in the set, attaches per-tool and plugin-level
// requirement configs when an agent_id is supplied. Returns ok=false
// when the tool-set's tools couldn't be loaded (caller skips it).
func (h *Handler) buildPluginResponse(
	r *http.Request, userID, agentID string, toolSet toolregistry.ToolSet,
) (PluginResponse, bool) {
	toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSet.ID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSet.ID, "error", err)
		return PluginResponse{}, false
	}
	toolConfigs := h.loadToolConfigs(r, userID, agentID, toolSet.ID)
	toolResponses := make([]common.ToolInSetResponse, 0, len(toolsInSet))
	for _, tool := range toolsInSet {
		toolResponses = append(toolResponses, buildToolInSetResponse(tool, toolConfigs))
	}
	pr := PluginResponse{
		ID:          toolSet.ID,
		PluginID:    toolregistry.GetPluginIDForSet(toolSet.ID),
		Name:        toolSet.Name,
		Description: toolSet.Description,
		Category:    toolSet.Category,
		Icon:        toolregistry.GetPluginIcon(toolSet.ID),
		Tools:       toolResponses,
	}
	if cfg := h.loadToolSetConfig(r, userID, agentID, toolSet.ID); cfg != nil {
		pr.RequirementConfig = cfg
	}
	return pr, true
}

// loadToolConfigs fetches per-tool requirement configs for an agent/tool-set
// pair, or returns nil when no agent context or service is set.
func (h *Handler) loadToolConfigs(r *http.Request, userID, agentID, toolSetID string) map[string]string {
	if agentID == "" || h.requirementConfigService == nil {
		return nil
	}
	configs, _ := h.requirementConfigService.ListToolRequirementConfigsJSON(r.Context(), userID, agentID, toolSetID)
	return configs
}

// loadToolSetConfig fetches the plugin-level (tool-set-level) requirement
// config for an agent/tool-set pair, parsing the JSON. Returns nil when
// no agent context, no service, empty config, or malformed JSON.
func (h *Handler) loadToolSetConfig(r *http.Request, userID, agentID, toolSetID string) map[string]any {
	if agentID == "" || h.requirementConfigService == nil {
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

// buildToolInSetResponse renders one tool's ToolInSetResponse, attaching
// its requirement config when one was supplied in the configs map.
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
