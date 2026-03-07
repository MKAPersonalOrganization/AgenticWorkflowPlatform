package taskmgmt

import (
	"encoding/json"
	"net/http"

	"school-with-ai/platform/toolregistry"
)

// PluginResponse represents a plugin (tool set) in the simplified API.
type PluginResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    string              `json:"category"`
	Icon        string              `json:"icon"`
	Enabled     bool                `json:"enabled"`
	Tools       []ToolInSetResponse `json:"tools"`
}

// ListPlugins returns all available plugins with their enabled state.
// GET /v1/plugins (global — no per-agent state)
// GET /v1/plugins?agent_id=xxx (per-agent enabled state for agent config)
func (h *TaskManagementHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// agent_id is optional — if absent, return plugins without per-agent enabled state
	agentID := r.URL.Query().Get("agent_id")

	allToolSets, err := h.toolRegistry.ListToolSets(r.Context())
	if err != nil {
		h.log.Error(r.Context(), "Failed to list tool sets", "error", err)
		http.Error(w, "Failed to retrieve plugins", http.StatusInternalServerError)
		return
	}

	// If an agent_id is provided, resolve per-agent enabled state
	enabledToolSets := make(map[string]bool)
	if agentID != "" {
		enabledToolSetIDs, err := h.permissionsService.GetEnabledToolSets(r.Context(), user.ID, agentID)
		if err != nil {
			h.log.Error(r.Context(), "Failed to get enabled tool sets", "agent_id", agentID, "error", err)
			// Continue without enabled state
		} else {
			for _, toolSetID := range enabledToolSetIDs {
				enabledToolSets[toolSetID] = true
			}
		}
	}

	plugins := make([]PluginResponse, 0, len(allToolSets))
	for _, toolSet := range allToolSets {
		toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSet.ID)
		if err != nil {
			h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSet.ID, "error", err)
			continue
		}

		toolResponses := make([]ToolInSetResponse, 0, len(toolsInSet))
		for _, tool := range toolsInSet {
			toolEnabled := false
			if agentID != "" {
				toolEnabled = h.permissionsService.HasPermission(r.Context(), user.ID, agentID, toolSet.ID, tool.ID)
			}
			toolResponses = append(toolResponses, ToolInSetResponse{
				ID:          tool.ID,
				Name:        tool.Name,
				Description: tool.Description,
				Category:    tool.Category,
				Enabled:     toolEnabled,
			})
		}

		plugins = append(plugins, PluginResponse{
			ID:          toolSet.ID,
			Name:        toolSet.Name,
			Description: toolSet.Description,
			Category:    toolSet.Category,
			Icon:        toolregistry.GetPluginIcon(toolSet.ID),
			Enabled:     enabledToolSets[toolSet.ID],
			Tools:       toolResponses,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"plugins": plugins}); err != nil {
		h.log.Error(r.Context(), "failed to encode plugins list", "error", err)
	}
}

// TogglePlugin enables or disables a plugin for a specific agent.
// PUT /v1/plugins/{pluginId}?agent_id=xxx (agent_id required)
func (h *TaskManagementHandler) TogglePlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("pluginId")
	if pluginID == "" {
		http.Error(w, "pluginId is required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id query parameter is required", http.StatusBadRequest)
		return
	}

	var req ToggleAgentToolSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := h.toolRegistry.GetToolSet(r.Context(), pluginID); err != nil {
		h.log.Error(r.Context(), "Plugin not found in registry", "plugin_id", pluginID, "error", err)
		http.Error(w, "Plugin not found", http.StatusNotFound)
		return
	}

	if err := h.permissionsService.SetToolSetPermission(r.Context(), user.ID, agentID, pluginID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update plugin permission",
			"agent_id", agentID, "plugin_id", pluginID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update plugin permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":   "success",
		"pluginId": pluginID,
		"enabled":  req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle plugin response", "error", err)
	}
}

// TogglePluginTool enables or disables an individual tool within a plugin for a specific agent.
// PUT /v1/plugins/{pluginId}/tools/{toolId}?agent_id=xxx (agent_id required)
func (h *TaskManagementHandler) TogglePluginTool(w http.ResponseWriter, r *http.Request) {
	pluginID := r.PathValue("pluginId")
	toolID := r.PathValue("toolId")

	if pluginID == "" || toolID == "" {
		http.Error(w, "pluginId and toolId are required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		http.Error(w, "agent_id query parameter is required", http.StatusBadRequest)
		return
	}

	var req ToggleIndividualToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if _, err := h.toolRegistry.GetToolSet(r.Context(), pluginID); err != nil {
		h.log.Error(r.Context(), "Plugin not found in registry", "plugin_id", pluginID, "error", err)
		http.Error(w, "Plugin not found", http.StatusNotFound)
		return
	}

	toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), pluginID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tools in plugin", "plugin_id", pluginID, "error", err)
		http.Error(w, "Failed to verify tool", http.StatusInternalServerError)
		return
	}

	toolExists := false
	for _, tool := range toolsInSet {
		if tool.ID == toolID {
			toolExists = true
			break
		}
	}
	if !toolExists {
		http.Error(w, "Tool not found in plugin", http.StatusNotFound)
		return
	}

	if err := h.permissionsService.SetPermission(r.Context(), user.ID, agentID, pluginID, toolID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update tool permission",
			"agent_id", agentID, "plugin_id", pluginID, "tool_id", toolID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update tool permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":   "success",
		"pluginId": pluginID,
		"toolId":   toolID,
		"enabled":  req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle plugin tool response", "error", err)
	}
}
