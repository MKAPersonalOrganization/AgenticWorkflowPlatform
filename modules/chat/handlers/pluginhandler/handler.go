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

	// agent_id is optional — if absent, return plugins without per-agent configs
	agentID := r.URL.Query().Get("agent_id")

	allToolSets, err := h.toolRegistry.ListToolSets(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "Failed to list tool sets", "error", err)
		http.Error(w, "Failed to retrieve plugins", http.StatusInternalServerError)
		return
	}

	plugins := make([]PluginResponse, 0, len(allToolSets))
	for _, toolSet := range allToolSets {
		toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSet.ID)
		if err != nil {
			h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSet.ID, "error", err)
			continue
		}

		// Load requirement configs for all tools in this set
		var toolConfigs map[string]string
		if agentID != "" && h.requirementConfigService != nil {
			toolConfigs, _ = h.requirementConfigService.ListToolRequirementConfigsJSON(r.Context(), user.ID, agentID, toolSet.ID)
		}

		toolResponses := make([]common.ToolInSetResponse, 0, len(toolsInSet))
		for _, tool := range toolsInSet {
			tr := common.ToolInSetResponse{
				ID:          tool.ID,
				Name:        tool.Name,
				Description: tool.Description,
				Category:    tool.Category,
			}

			// Attach requirement config if present
			if cfgJSON, ok := toolConfigs[tool.ID]; ok && cfgJSON != "" && cfgJSON != "{}" {
				var cfg map[string]any
				if err := json.Unmarshal([]byte(cfgJSON), &cfg); err == nil {
					tr.RequirementConfig = cfg
				}
			}

			toolResponses = append(toolResponses, tr)
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

		// Load plugin-level requirement config
		if agentID != "" && h.requirementConfigService != nil {
			tsConfigJSON, _ := h.requirementConfigService.GetToolSetRequirementConfigJSON(r.Context(), user.ID, agentID, toolSet.ID)
			if tsConfigJSON != "" && tsConfigJSON != "{}" {
				var tsCfg map[string]any
				if err := json.Unmarshal([]byte(tsConfigJSON), &tsCfg); err == nil {
					pr.RequirementConfig = tsCfg
				}
			}
		}

		plugins = append(plugins, pr)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"plugins": plugins}); err != nil {
		h.log.Error(r.Context(), "failed to encode plugins list", "error", err)
	}
}
