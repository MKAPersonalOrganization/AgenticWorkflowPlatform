package mcphandler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"school-with-ai/domain"
	"school-with-ai/foundation/i18n"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/plugindiscovery"

	"github.com/go-chi/chi/v5"
)

// Handler manages MCP connector HTTP requests.
type Handler struct {
	log     *logger.Logger
	service *plugindiscovery.Service
}

// NewHandler creates a new MCP handler.
func NewHandler(log *logger.Logger, service *plugindiscovery.Service) *Handler {
	return &Handler{log: log, service: service}
}

// --------------------------------------------------------------------------
// Request/Response types
// --------------------------------------------------------------------------

type createConnectorRequest struct {
	ServerURL         string `json:"server_url"`
	Title             string `json:"title"`
	OAuthClientID     string `json:"oauth_client_id,omitempty"`
	OAuthClientSecret string `json:"oauth_client_secret,omitempty"`
}

type connectorResponse struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	ServerURL     string `json:"server_url"`
	Status        string `json:"status"`
	ErrorMsg      string `json:"error_msg,omitempty"`
	ServerName    string `json:"server_name,omitempty"`
	ServerVersion string `json:"server_version,omitempty"`
	ToolCount     int    `json:"tool_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type connectResponse struct {
	Connector *connectorResponse `json:"connector"`
	AuthURL   string             `json:"auth_url,omitempty"` // Set if OAuth is needed
}

type toolResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"`
	Annotations any    `json:"annotations"`
}

func toConnectorResponse(c domain.MCPConnector, toolCount int) connectorResponse {
	resp := connectorResponse{
		ID:        c.ID,
		Title:     c.Title,
		ServerURL: c.ServerURL,
		Status:    string(c.Status),
		ErrorMsg:  c.ErrorMsg,
		ToolCount: toolCount,
		CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if c.ServerName.Valid {
		resp.ServerName = c.ServerName.String
	}
	if c.ServerVersion.Valid {
		resp.ServerVersion = c.ServerVersion.String
	}
	return resp
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

// ListConnectors handles GET /v1/mcp/connectors
func (h *Handler) ListConnectors(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	connectors, err := h.service.ListConnectors(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list connectors", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to list MCP connectors"), http.StatusInternalServerError)
		return
	}

	result := make([]connectorResponse, 0, len(connectors))
	for _, c := range connectors {
		tools, _ := h.service.GetConnectorTools(r.Context(), c.ID)
		result = append(result, toConnectorResponse(c, len(tools)))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"connectors": result})
}

// CreateConnector handles POST /v1/mcp/connectors
func (h *Handler) CreateConnector(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	var req createConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	if req.ServerURL == "" {
		http.Error(w, i18n.T(r.Context(), "Server URL is required"), http.StatusBadRequest)
		return
	}

	connector, err := h.service.CreateConnector(r.Context(), user.ID, req.ServerURL, req.Title)
	if err != nil {
		h.log.Error(r.Context(), "failed to create connector", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If OAuth credentials were provided, set them before any future connect call.
	if req.OAuthClientID != "" {
		connector.OAuthClientID = sql.NullString{String: req.OAuthClientID, Valid: true}
		connector.OAuthClientSecret = sql.NullString{String: req.OAuthClientSecret, Valid: req.OAuthClientSecret != ""}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toConnectorResponse(*connector, 0))
}

// GetConnector handles GET /v1/mcp/connectors/{id}
func (h *Handler) GetConnector(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	connector, err := h.service.GetConnector(r.Context(), id)
	if err != nil {
		h.log.Error(r.Context(), "failed to get connector", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to get connector"), http.StatusInternalServerError)
		return
	}
	if connector == nil || connector.UserID != user.ID {
		http.Error(w, i18n.T(r.Context(), "Connector not found"), http.StatusNotFound)
		return
	}

	tools, _ := h.service.GetConnectorTools(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConnectorResponse(*connector, len(tools)))
}

// DeleteConnector handles DELETE /v1/mcp/connectors/{id}
func (h *Handler) DeleteConnector(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.DeleteConnector(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to delete connector", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Connect handles POST /v1/mcp/connectors/{id}/connect
func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	// Verify ownership.
	connector, err := h.service.GetConnector(r.Context(), id)
	if err != nil || connector == nil || connector.UserID != user.ID {
		http.Error(w, i18n.T(r.Context(), "Connector not found"), http.StatusNotFound)
		return
	}

	updated, authURL, err := h.service.ConnectConnector(r.Context(), id)
	if err != nil {
		h.log.Error(r.Context(), "failed to connect", "error", err, "connector_id", id)
		// Return the connector status even on error.
		if updated != nil {
			tools, _ := h.service.GetConnectorTools(r.Context(), id)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(connectResponse{
				Connector: ptr(toConnectorResponse(*updated, len(tools))),
				AuthURL:   authURL,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tools, _ := h.service.GetConnectorTools(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connectResponse{
		Connector: ptr(toConnectorResponse(*updated, len(tools))),
		AuthURL:   authURL,
	})
}

// Disconnect handles POST /v1/mcp/connectors/{id}/disconnect
func (h *Handler) Disconnect(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	connector, err := h.service.GetConnector(r.Context(), id)
	if err != nil || connector == nil || connector.UserID != user.ID {
		http.Error(w, i18n.T(r.Context(), "Connector not found"), http.StatusNotFound)
		return
	}

	if err := h.service.DisconnectConnector(r.Context(), id); err != nil {
		h.log.Error(r.Context(), "failed to disconnect", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListTools handles GET /v1/mcp/connectors/{id}/tools
func (h *Handler) ListTools(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	connector, err := h.service.GetConnector(r.Context(), id)
	if err != nil || connector == nil || connector.UserID != user.ID {
		http.Error(w, i18n.T(r.Context(), "Connector not found"), http.StatusNotFound)
		return
	}

	tools, err := h.service.GetConnectorTools(r.Context(), id)
	if err != nil {
		h.log.Error(r.Context(), "failed to list tools", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to list tools"), http.StatusInternalServerError)
		return
	}

	result := make([]toolResponse, 0, len(tools))
	for _, t := range tools {
		var schema any
		json.Unmarshal([]byte(t.InputSchema), &schema)
		var annotations any
		json.Unmarshal([]byte(t.Annotations), &annotations)

		result = append(result, toolResponse{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
			Annotations: annotations,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tools": result})
}

// RefreshTools handles POST /v1/mcp/connectors/{id}/refresh
func (h *Handler) RefreshTools(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	connector, err := h.service.GetConnector(r.Context(), id)
	if err != nil || connector == nil || connector.UserID != user.ID {
		http.Error(w, i18n.T(r.Context(), "Connector not found"), http.StatusNotFound)
		return
	}

	tools, err := h.service.RefreshTools(r.Context(), id)
	if err != nil {
		h.log.Error(r.Context(), "failed to refresh tools", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]toolResponse, 0, len(tools))
	for _, t := range tools {
		var schema any
		json.Unmarshal([]byte(t.InputSchema), &schema)
		var annotations any
		json.Unmarshal([]byte(t.Annotations), &annotations)

		result = append(result, toolResponse{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
			Annotations: annotations,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"tools": result})
}

// OAuthCallback handles GET /v1/mcp/oauth/callback
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if state == "" || code == "" {
		http.Error(w, "Missing state or code parameter", http.StatusBadRequest)
		return
	}

	connector, err := h.service.HandleOAuthCallback(r.Context(), state, code)
	if err != nil {
		h.log.Error(r.Context(), "OAuth callback failed", "error", err)
		// Return an HTML page that communicates the error to the opener.
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body><script>
			window.opener && window.opener.postMessage({type:'mcp-oauth-callback',success:false,error:` + jsonEscape(err.Error()) + `},'*');
			window.close();
		</script><p>Authentication failed. You can close this window.</p></body></html>`))
		return
	}

	// Return an HTML page that communicates success to the opener.
	connectorJSON, _ := json.Marshal(map[string]any{
		"type":         "mcp-oauth-callback",
		"success":      true,
		"connector_id": connector.ID,
		"status":       string(connector.Status),
	})

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html><html><body><script>
		window.opener && window.opener.postMessage(` + string(connectorJSON) + `,'*');
		window.close();
	</script><p>Authentication successful! You can close this window.</p></body></html>`))
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func ptr[T any](v T) *T {
	return &v
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
