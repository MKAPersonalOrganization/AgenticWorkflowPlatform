package toolsethandler

import (
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/chat/handlers/common"
	"aicompass/foundation/auth"
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

	// Get all tool sets from registry
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

	// Convert to response format
	responseToolSets := make([]AgentToolSetResponse, 0, len(allToolSets))
	for _, toolSet := range allToolSets {
		// Get tools in this tool set
		toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSet.ID)
		if err != nil {
			h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSet.ID, "error", err)
			continue
		}

		// Load requirement configs for all tools in this set
		var toolConfigs map[string]string
		if h.requirementConfigService != nil {
			toolConfigs, _ = h.requirementConfigService.ListToolRequirementConfigsJSON(r.Context(), user.ID, agentID, toolSet.ID)
		}

		// Convert tools to response format with requirement configs
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

		// Build tool set response
		tsResp := AgentToolSetResponse{
			ID:          toolSet.ID,
			Name:        toolSet.Name,
			Description: toolSet.Description,
			Category:    toolSet.Category,
			Tools:       toolResponses,
		}

		// Load tool set level requirement config
		if h.requirementConfigService != nil {
			tsConfigJSON, _ := h.requirementConfigService.GetToolSetRequirementConfigJSON(r.Context(), user.ID, agentID, toolSet.ID)
			if tsConfigJSON != "" && tsConfigJSON != "{}" {
				var tsCfg map[string]any
				if err := json.Unmarshal([]byte(tsConfigJSON), &tsCfg); err == nil {
					tsResp.RequirementConfig = tsCfg
				}
			}
		}

		responseToolSets = append(responseToolSets, tsResp)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"toolsets": responseToolSets}); err != nil {
		h.log.Error(r.Context(), "failed to encode tool sets list", "error", err)
	}
}
