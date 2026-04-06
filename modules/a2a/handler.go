package a2a

import (
	"encoding/json"
	"errors"
	"net/http"

	"aicompass/foundation/i18n"
	"aicompass/foundation/logger"
	"aicompass/shared"

	"github.com/go-chi/chi/v5"
)

// handler manages A2A connector HTTP requests.
type handler struct {
	log     *logger.Logger
	service *Service
}

func newHandler(log *logger.Logger, service *Service) *handler {
	return &handler{log: log, service: service}
}

// --------------------------------------------------------------------------
// Request/Response types
// --------------------------------------------------------------------------

type createConnectorRequest struct {
	AgentURL   string `json:"agent_url"`
	Title      string `json:"title"`
	AuthType   string `json:"auth_type,omitempty"` // "none", "bearer", "api_key", "custom_header"
	AuthToken  string `json:"auth_token,omitempty"`
	AuthHeader string `json:"auth_header,omitempty"` // Custom header name
}

type connectorResponse struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	AgentURL            string `json:"agent_url"`
	Status              string `json:"status"`
	ErrorMsg            string `json:"error_msg,omitempty"`
	DisconnectReason    string `json:"disconnect_reason,omitempty"`
	DisconnectPermanent bool   `json:"disconnect_permanent,omitempty"`
	AgentName           string `json:"agent_name,omitempty"`
	AgentDescription    string `json:"agent_description,omitempty"`
	AgentVersion        string `json:"agent_version,omitempty"`
	IconURL             string `json:"icon_url,omitempty"`
	Capabilities        string `json:"capabilities,omitempty"`
	SkillCount          int    `json:"skill_count"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type skillResponse struct {
	ID          string   `json:"id"`
	SkillID     string   `json:"skill_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples"`
}

func toConnectorResponse(c A2AConnector, skillCount int) connectorResponse {
	resp := connectorResponse{
		ID:                  c.ID,
		Title:               c.Title,
		AgentURL:            c.AgentURL,
		Status:              string(c.Status),
		ErrorMsg:            c.ErrorMsg,
		DisconnectReason:    c.DisconnectReason,
		DisconnectPermanent: c.DisconnectPermanent,
		SkillCount:          skillCount,
		CreatedAt:           c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:           c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	resp.AgentName = c.AgentName
	resp.AgentDescription = c.AgentDescription
	resp.AgentVersion = c.AgentVersion
	resp.IconURL = c.IconURL
	resp.Capabilities = c.Capabilities
	return resp
}

func toSkillResponse(s A2ADiscoveredSkill) skillResponse {
	var tags, examples []string
	json.Unmarshal([]byte(s.Tags), &tags)         //nolint:errcheck
	json.Unmarshal([]byte(s.Examples), &examples) //nolint:errcheck

	return skillResponse{
		ID:          s.ID,
		SkillID:     s.SkillID,
		Name:        s.Name,
		Description: s.Description,
		Tags:        tags,
		Examples:    examples,
	}
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

// listConnectors handles GET /v1/a2a/connectors
func (h *handler) listConnectors(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	connectors, err := h.service.ListConnectors(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list a2a connectors", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to list A2A agents"), http.StatusInternalServerError)
		return
	}

	ids := make([]string, len(connectors))
	for i, c := range connectors {
		ids[i] = c.ID
	}
	skillCounts, _ := h.service.CountSkillsByConnectors(r.Context(), ids)

	result := make([]connectorResponse, 0, len(connectors))
	for _, c := range connectors {
		result = append(result, toConnectorResponse(c, skillCounts[c.ID]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"connectors": result})
}

// createConnector handles POST /v1/a2a/connectors
func (h *handler) createConnector(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	var req createConnectorRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	if req.AgentURL == "" {
		http.Error(w, i18n.T(r.Context(), "Agent URL is required"), http.StatusBadRequest)
		return
	}

	connector, err := h.service.CreateConnector(r.Context(), user.ID, req.AgentURL, req.Title, req.AuthType, req.AuthToken, req.AuthHeader)
	if err != nil {
		h.log.Error(r.Context(), "failed to create a2a connector", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toConnectorResponse(*connector, 0))
}

// getConnector handles GET /v1/a2a/connectors/{id}
func (h *handler) getConnector(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	connector, err := h.service.GetConnector(r.Context(), id, user.ID)
	if err != nil {
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	skills, _ := h.service.GetConnectorSkills(r.Context(), id, user.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConnectorResponse(*connector, len(skills)))
}

// deleteConnector handles DELETE /v1/a2a/connectors/{id}
func (h *handler) deleteConnector(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.DeleteConnector(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to delete a2a connector", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// connect handles POST /v1/a2a/connectors/{id}/connect
func (h *handler) connect(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	updated, err := h.service.ConnectConnector(r.Context(), id, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to connect a2a agent", "error", err, "connector_id", id)
		if updated != nil {
			skills, _ := h.service.GetConnectorSkills(r.Context(), id, user.ID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(toConnectorResponse(*updated, len(skills)))
			return
		}
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	skills, _ := h.service.GetConnectorSkills(r.Context(), id, user.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConnectorResponse(*updated, len(skills)))
}

// disconnect handles POST /v1/a2a/connectors/{id}/disconnect
func (h *handler) disconnect(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	updated, err := h.service.DisconnectConnector(r.Context(), id, user.ID, "user requested disconnect")
	if err != nil {
		h.log.Error(r.Context(), "failed to disconnect a2a agent", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConnectorResponse(*updated, 0))
}

// listSkills handles GET /v1/a2a/connectors/{id}/skills
func (h *handler) listSkills(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	skills, err := h.service.GetConnectorSkills(r.Context(), id, user.ID)
	if err != nil {
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	result := make([]skillResponse, 0, len(skills))
	for _, s := range skills {
		result = append(result, toSkillResponse(s))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"skills": result})
}

// refreshSkills handles POST /v1/a2a/connectors/{id}/refresh
func (h *handler) refreshSkills(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	updated, err := h.service.RefreshSkills(r.Context(), id, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to refresh a2a skills", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	skills, _ := h.service.GetConnectorSkills(r.Context(), id, user.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConnectorResponse(*updated, len(skills)))
}

// --------------------------------------------------------------------------
// Error mapping
// --------------------------------------------------------------------------

func mapServiceError(err error) (string, int) {
	switch {
	case errors.Is(err, ErrConnectorNotFound):
		return "A2A connector not found", http.StatusNotFound
	case errors.Is(err, ErrDuplicateConnector):
		return "An A2A connector for this URL already exists", http.StatusConflict
	case errors.Is(err, ErrConnectorLimitReached):
		return "Maximum number of A2A connectors reached", http.StatusTooManyRequests
	case errors.Is(err, ErrInvalidConnectorState):
		return "Connector is not in a valid state for this operation", http.StatusConflict
	case errors.Is(err, ErrConnectorNotConnected):
		return "A2A connector is not connected", http.StatusServiceUnavailable
	case errors.Is(err, ErrAgentCardNotFound):
		return "Could not discover agent card at the given URL", http.StatusUnprocessableEntity
	case errors.Is(err, ErrAgentCardInvalid):
		return "Agent card is invalid or missing required fields", http.StatusUnprocessableEntity
	default:
		return "Internal server error", http.StatusInternalServerError
	}
}

func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
