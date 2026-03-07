package taskmgmt

import (
	"encoding/json"
	"net/http"
)

// AgentToolSetResponse represents the API response format for agent tool sets.
type AgentToolSetResponse struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    string              `json:"category"`
	Enabled     bool                `json:"enabled"`
	Tools       []ToolInSetResponse `json:"tools"`
}

// ToolInSetResponse represents individual tools within a tool set.
type ToolInSetResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"` // Individual tool permission status
}

// ListAgentToolSets returns a list of tool sets available to an agent.
// GET /v1/agents/{agentId}/toolsets
func (h *TaskManagementHandler) ListAgentToolSets(w http.ResponseWriter, r *http.Request) {
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

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get enabled tool set IDs for the agent
	enabledToolSetIDs, err := h.permissionsService.GetEnabledToolSets(r.Context(), user.ID, agentID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get enabled tool sets", "agent_id", agentID, "error", err)
		http.Error(w, "Failed to retrieve agent tool set permissions", http.StatusInternalServerError)
		return
	}

	// Create a map of enabled tool sets for quick lookup
	enabledToolSets := make(map[string]bool)
	for _, toolSetID := range enabledToolSetIDs {
		enabledToolSets[toolSetID] = true
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

		// Convert tools to response format with individual permissions
		toolResponses := make([]ToolInSetResponse, 0, len(toolsInSet))
		for _, tool := range toolsInSet {
			// Check individual tool permission - if no explicit permission, inherit from tool set
			toolEnabled := h.permissionsService.HasPermission(r.Context(), user.ID, agentID, toolSet.ID, tool.ID)

			toolResponses = append(toolResponses, ToolInSetResponse{
				ID:          tool.ID,
				Name:        tool.Name,
				Description: tool.Description,
				Category:    tool.Category,
				Enabled:     toolEnabled,
			})
		}

		responseToolSets = append(responseToolSets, AgentToolSetResponse{
			ID:          toolSet.ID,
			Name:        toolSet.Name,
			Description: toolSet.Description,
			Category:    toolSet.Category,
			Enabled:     enabledToolSets[toolSet.ID],
			Tools:       toolResponses,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"toolsets": responseToolSets}); err != nil {
		h.log.Error(r.Context(), "failed to encode tool sets list", "error", err)
	}
}

// ToggleAgentToolSetRequest represents the request body for toggling a tool set.
type ToggleAgentToolSetRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleAgentToolSet enables or disables a tool set for an agent.
// PUT /v1/agents/{agentId}/toolsets/{toolSetId}
func (h *TaskManagementHandler) ToggleAgentToolSet(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")
	if agentID == "" || toolSetID == "" {
		http.Error(w, "agentId and toolSetId are required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ToggleAgentToolSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify tool set exists in registry
	if _, err := h.toolRegistry.GetToolSet(r.Context(), toolSetID); err != nil {
		h.log.Error(r.Context(), "Tool set not found in registry",
			"tool_set_id", toolSetID, "error", err)
		http.Error(w, "Tool set not found", http.StatusNotFound)
		return
	}

	// Update permission in permissions service
	if err := h.permissionsService.SetToolSetPermission(r.Context(), user.ID, agentID, toolSetID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update agent tool set permission",
			"agent_id", agentID, "tool_set_id", toolSetID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update tool set permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":    "success",
		"toolSetId": toolSetID,
		"enabled":   req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle tool set response", "error", err)
	}
}

// ToggleIndividualToolRequest represents the request body for toggling an individual tool within a tool set.
type ToggleIndividualToolRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleIndividualTool enables or disables an individual tool within a tool set for an agent.
// PUT /v1/agents/{agentId}/toolsets/{toolSetId}/tools/{toolId}
func (h *TaskManagementHandler) ToggleIndividualTool(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")
	toolID := r.PathValue("toolId")

	if agentID == "" || toolSetID == "" || toolID == "" {
		http.Error(w, "agentId, toolSetId, and toolId are required", http.StatusBadRequest)
		return
	}

	user := h.authService.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ToggleIndividualToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify tool set exists in registry
	if _, err := h.toolRegistry.GetToolSet(r.Context(), toolSetID); err != nil {
		h.log.Error(r.Context(), "Tool set not found in registry",
			"tool_set_id", toolSetID, "error", err)
		http.Error(w, "Tool set not found", http.StatusNotFound)
		return
	}

	// Verify tool exists in the tool set
	toolsInSet, err := h.toolRegistry.GetToolsInSet(r.Context(), toolSetID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tools in set", "tool_set_id", toolSetID, "error", err)
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
		http.Error(w, "Tool not found in tool set", http.StatusNotFound)
		return
	}

	// Update individual tool permission
	if err := h.permissionsService.SetPermission(r.Context(), user.ID, agentID, toolSetID, toolID, req.Enabled); err != nil {
		h.log.Error(r.Context(), "Failed to update individual tool permission",
			"agent_id", agentID, "tool_set_id", toolSetID, "tool_id", toolID, "enabled", req.Enabled, "error", err)
		http.Error(w, "Failed to update tool permission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status":    "success",
		"toolSetId": toolSetID,
		"toolId":    toolID,
		"enabled":   req.Enabled,
	}); err != nil {
		h.log.Error(r.Context(), "failed to encode toggle individual tool response", "error", err)
	}
}
