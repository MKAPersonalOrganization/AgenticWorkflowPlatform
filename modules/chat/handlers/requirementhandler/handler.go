package requirementhandler

import (
	"aicompass/foundation/logger"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/foundation/auth"
	"encoding/json"
	"io"
	"net/http"
)

// Handler handles tool requirement configuration HTTP endpoints.
type Handler struct {
	log                      *logger.Logger
	requirementConfigService chatcontracts.ToolRequirementConfigService
}

// New creates a new requirement handler.
func New(
	log *logger.Logger,
	requirementConfigService chatcontracts.ToolRequirementConfigService,
) *Handler {
	return &Handler{
		log:                      log,
		requirementConfigService: requirementConfigService,
	}
}

// ToolRequirementConfigResponse is the JSON shape returned to the frontend.
type ToolRequirementConfigResponse struct {
	ToolSetID           string `json:"tool_set_id"`
	ToolID              string `json:"tool_id,omitempty"`
	Action              string `json:"action"`
	RequireConfirmation bool   `json:"require_confirmation"`
	ConfirmationHint    string `json:"confirmation_hint,omitempty"`
}

// ToolRequirementConfigRequest is the JSON shape accepted from the frontend.
type ToolRequirementConfigRequest struct {
	Action              string `json:"action"`
	RequireConfirmation bool   `json:"require_confirmation"`
	ConfirmationHint    string `json:"confirmation_hint,omitempty"`
}

// GetToolRequirementConfig returns the requirement configuration for a single tool.
// GET /v1/agents/{agentId}/toolsets/{toolSetId}/tools/{toolId}/requirements
func (h *Handler) GetToolRequirementConfig(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")
	toolID := r.PathValue("toolId")

	if agentID == "" || toolSetID == "" || toolID == "" {
		http.Error(w, "agentId, toolSetId, and toolId are required", http.StatusBadRequest)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.requirementConfigService == nil {
		http.Error(w, "Requirement config service not available", http.StatusServiceUnavailable)
		return
	}

	cfgJSON, err := h.requirementConfigService.GetToolRequirementConfigJSON(r.Context(), user.ID, agentID, toolSetID, toolID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tool requirement config",
			"agent_id", agentID, "tool_set_id", toolSetID, "tool_id", toolID, "error", err)
		http.Error(w, "Failed to retrieve requirement config", http.StatusInternalServerError)
		return
	}

	// Parse the JSON config to build a typed response
	var resp ToolRequirementConfigResponse
	if cfgJSON != "" && cfgJSON != "{}" {
		if err := json.Unmarshal([]byte(cfgJSON), &resp); err != nil {
			resp = ToolRequirementConfigResponse{}
		}
	}
	resp.ToolSetID = toolSetID
	resp.ToolID = toolID

	// Default action to "allow" for display
	if resp.Action == "" {
		resp.Action = "allow"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SetToolRequirementConfig updates the requirement configuration for a single tool.
// PUT /v1/agents/{agentId}/toolsets/{toolSetId}/tools/{toolId}/requirements
func (h *Handler) SetToolRequirementConfig(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")
	toolID := r.PathValue("toolId")

	if agentID == "" || toolSetID == "" || toolID == "" {
		http.Error(w, "agentId, toolSetId, and toolId are required", http.StatusBadRequest)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.requirementConfigService == nil {
		http.Error(w, "Requirement config service not available", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate the request shape
	var req ToolRequirementConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	// Validate action value
	switch req.Action {
	case "allow", "block", "require_confirmation", "":
		// valid
	default:
		http.Error(w, "Invalid action: must be 'allow', 'block', or 'require_confirmation'", http.StatusBadRequest)
		return
	}

	if err := h.requirementConfigService.SetToolRequirementConfigJSON(r.Context(), user.ID, agentID, toolSetID, toolID, string(body)); err != nil {
		h.log.Error(r.Context(), "Failed to set tool requirement config",
			"agent_id", agentID, "tool_set_id", toolSetID, "tool_id", toolID, "error", err)
		http.Error(w, "Failed to save requirement config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "success",
		"tool_set_id": toolSetID,
		"tool_id":     toolID,
		"config":      req,
	})
}

// GetToolSetRequirementConfig returns the requirement configuration for a tool set.
// GET /v1/agents/{agentId}/toolsets/{toolSetId}/requirements
func (h *Handler) GetToolSetRequirementConfig(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")

	if agentID == "" || toolSetID == "" {
		http.Error(w, "agentId and toolSetId are required", http.StatusBadRequest)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.requirementConfigService == nil {
		http.Error(w, "Requirement config service not available", http.StatusServiceUnavailable)
		return
	}

	cfgJSON, err := h.requirementConfigService.GetToolSetRequirementConfigJSON(r.Context(), user.ID, agentID, toolSetID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to get tool set requirement config",
			"agent_id", agentID, "tool_set_id", toolSetID, "error", err)
		http.Error(w, "Failed to retrieve requirement config", http.StatusInternalServerError)
		return
	}

	var resp ToolRequirementConfigResponse
	if cfgJSON != "" && cfgJSON != "{}" {
		if err := json.Unmarshal([]byte(cfgJSON), &resp); err != nil {
			resp = ToolRequirementConfigResponse{}
		}
	}
	resp.ToolSetID = toolSetID

	if resp.Action == "" {
		resp.Action = "allow"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SetToolSetRequirementConfig updates the requirement configuration for a tool set.
// PUT /v1/agents/{agentId}/toolsets/{toolSetId}/requirements
func (h *Handler) SetToolSetRequirementConfig(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")

	if agentID == "" || toolSetID == "" {
		http.Error(w, "agentId and toolSetId are required", http.StatusBadRequest)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.requirementConfigService == nil {
		http.Error(w, "Requirement config service not available", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var req ToolRequirementConfigRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "allow", "block", "require_confirmation", "":
		// valid
	default:
		http.Error(w, "Invalid action: must be 'allow', 'block', or 'require_confirmation'", http.StatusBadRequest)
		return
	}

	if err := h.requirementConfigService.SetToolSetRequirementConfigJSON(r.Context(), user.ID, agentID, toolSetID, string(body)); err != nil {
		h.log.Error(r.Context(), "Failed to set tool set requirement config",
			"agent_id", agentID, "tool_set_id", toolSetID, "error", err)
		http.Error(w, "Failed to save requirement config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":      "success",
		"tool_set_id": toolSetID,
		"config":      req,
	})
}

// ListToolRequirementConfigs returns all tool requirement configs for a tool set.
// GET /v1/agents/{agentId}/toolsets/{toolSetId}/tools/requirements
func (h *Handler) ListToolRequirementConfigs(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	toolSetID := r.PathValue("toolSetId")

	if agentID == "" || toolSetID == "" {
		http.Error(w, "agentId and toolSetId are required", http.StatusBadRequest)
		return
	}

	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.requirementConfigService == nil {
		http.Error(w, "Requirement config service not available", http.StatusServiceUnavailable)
		return
	}

	configsJSON, err := h.requirementConfigService.ListToolRequirementConfigsJSON(r.Context(), user.ID, agentID, toolSetID)
	if err != nil {
		h.log.Error(r.Context(), "Failed to list tool requirement configs",
			"agent_id", agentID, "tool_set_id", toolSetID, "error", err)
		http.Error(w, "Failed to retrieve requirement configs", http.StatusInternalServerError)
		return
	}

	// Convert JSON strings to structured response
	configs := make(map[string]ToolRequirementConfigResponse, len(configsJSON))
	for toolID, cfgStr := range configsJSON {
		var resp ToolRequirementConfigResponse
		if err := json.Unmarshal([]byte(cfgStr), &resp); err != nil {
			continue
		}
		resp.ToolSetID = toolSetID
		resp.ToolID = toolID
		if resp.Action == "" {
			resp.Action = "allow"
		}
		configs[toolID] = resp
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"configs": configs})
}
