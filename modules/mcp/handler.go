package mcp

import (
	"aicompass/foundation/i18n"
	"aicompass/foundation/logger"
	"aicompass/shared"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

// handler manages MCP connector HTTP requests.
type handler struct {
	log     *logger.Logger
	service *Service
}

// newHandler creates a new MCP handler.
func newHandler(log *logger.Logger, service *Service) *handler {
	return &handler{log: log, service: service}
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
	ID                  string `json:"id"`
	Title               string `json:"title"`
	ServerURL           string `json:"server_url"`
	Status              string `json:"status"`
	ErrorMsg            string `json:"error_msg,omitempty"`
	DisconnectReason    string `json:"disconnect_reason,omitempty"`
	DisconnectPermanent bool   `json:"disconnect_permanent,omitempty"`
	ServerName          string `json:"server_name,omitempty"`
	ServerVersion       string `json:"server_version,omitempty"`
	ToolCount           int    `json:"tool_count"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
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

type presetResponse struct {
	URL         string `json:"url"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func toConnectorResponse(c MCPConnector, toolCount int) connectorResponse {
	resp := connectorResponse{
		ID:                  c.ID,
		Title:               c.Title,
		ServerURL:           c.ServerURL,
		Status:              string(c.Status),
		ErrorMsg:            c.ErrorMsg,
		DisconnectReason:    c.DisconnectReason,
		DisconnectPermanent: c.DisconnectPermanent,
		ToolCount:           toolCount,
		CreatedAt:           c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:           c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
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

// listConnectors handles GET /v1/mcp/connectors
func (h *handler) listConnectors(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
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

	// Batch-fetch tool counts to avoid N+1 queries.
	ids := make([]string, len(connectors))
	for i, c := range connectors {
		ids[i] = c.ID
	}
	toolCounts, _ := h.service.CountToolsByConnectors(r.Context(), ids)

	result := make([]connectorResponse, 0, len(connectors))
	for _, c := range connectors {
		result = append(result, toConnectorResponse(c, toolCounts[c.ID]))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"connectors": result})
}

// createConnector handles POST /v1/mcp/connectors
func (h *handler) createConnector(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	var req createConnectorRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB limit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	if req.ServerURL == "" {
		http.Error(w, i18n.T(r.Context(), "Server URL is required"), http.StatusBadRequest)
		return
	}

	connector, err := h.service.CreateConnector(r.Context(), user.ID, req.ServerURL, req.Title, req.OAuthClientID, req.OAuthClientSecret)
	if err != nil {
		h.log.Error(r.Context(), "failed to create connector", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toConnectorResponse(*connector, 0))
}

// getConnector handles GET /v1/mcp/connectors/{id}
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

	tools, _ := h.service.GetConnectorTools(r.Context(), id, user.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConnectorResponse(*connector, len(tools)))
}

// deleteConnector handles DELETE /v1/mcp/connectors/{id}
func (h *handler) deleteConnector(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.DeleteConnector(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to delete connector", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// connect handles POST /v1/mcp/connectors/{id}/connect
func (h *handler) connect(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	// Read optional access_token from request body (agentstack pattern).
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
	}

	updated, authURL, err := h.service.ConnectConnector(r.Context(), id, user.ID, body.AccessToken)
	if err != nil {
		h.log.Error(r.Context(), "failed to connect", "error", err, "connector_id", id)
		// Return the connector status even on error so the UI can show the state.
		if updated != nil {
			tools, _ := h.service.GetConnectorTools(r.Context(), id, user.ID)
			setOAuthStateCookie(w, r, authURL)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(connectResponse{
				Connector: ptr(toConnectorResponse(*updated, len(tools))),
				AuthURL:   authURL,
			})
			return
		}
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	tools, _ := h.service.GetConnectorTools(r.Context(), id, user.ID)
	setOAuthStateCookie(w, r, authURL)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connectResponse{
		Connector: ptr(toConnectorResponse(*updated, len(tools))),
		AuthURL:   authURL,
	})
}

// disconnect handles POST /v1/mcp/connectors/{id}/disconnect
func (h *handler) disconnect(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	updated, err := h.service.DisconnectConnector(r.Context(), id, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to disconnect", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}

	tools, _ := h.service.GetConnectorTools(r.Context(), id, user.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toConnectorResponse(*updated, len(tools)))
}

// listTools handles GET /v1/mcp/connectors/{id}/tools
func (h *handler) listTools(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	tools, err := h.service.GetConnectorTools(r.Context(), id, user.ID)
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

// refreshTools handles POST /v1/mcp/connectors/{id}/refresh
func (h *handler) refreshTools(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	tools, err := h.service.RefreshTools(r.Context(), id, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to refresh tools", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
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

// listPresets handles GET /v1/mcp/connectors/presets
// Returns pre-configured connector templates for common MCP servers (agentstack pattern).
func (h *handler) listPresets(w http.ResponseWriter, r *http.Request) {
	presets := h.service.ListPresets()
	result := make([]presetResponse, 0, len(presets))
	for _, p := range presets {
		result = append(result, presetResponse{
			URL:         p.URL,
			Name:        p.Name,
			Description: p.Description,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"presets": result})
}

// mcpProxy handles all methods on /v1/mcp/connectors/{id}/mcp/*
// Acts as an authenticated reverse proxy to the connector's MCP server.
// This is agentstack's core pattern: the platform forwards raw MCP protocol
// messages so external clients can use connectors directly.
func (h *handler) mcpProxy(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")

	if err := h.service.MCPProxy(r.Context(), id, user.ID, w, r); err != nil {
		h.log.Error(r.Context(), "mcp proxy error", "error", err, "connector_id", id)
		// Don't write error if headers already sent (streaming started).
		if !headersSent(w) {
			msg, code := mapServiceError(err)
			writeJSONError(w, msg, code)
		}
	}
}

// oauthCallback handles GET /v1/mcp/oauth/callback
func (h *handler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	// Handle OAuth error response from the authorization server (RFC 6749 §4.1.2.1).
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		desc := r.URL.Query().Get("error_description")
		if desc == "" {
			desc = oauthErr
		}
		h.log.Error(r.Context(), "OAuth provider returned error",
			"error", oauthErr, "description", desc)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html><html><body><script>
window.opener && window.opener.postMessage({type:'mcp-oauth-callback',success:false,error:` + jsonEscape(desc) + `},window.location.origin);
window.close();
</script><p>Authentication failed: ` + desc + `. You can close this window.</p></body></html>`))
		return
	}

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
window.opener && window.opener.postMessage({type:'mcp-oauth-callback',success:false,error:` + jsonEscape(err.Error()) + `},window.location.origin);
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
window.opener && window.opener.postMessage(` + string(connectorJSON) + `,window.location.origin);
window.close();
</script><p>Authentication successful! You can close this window.</p></body></html>`))
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func ptr[T any](v T) *T {
	return &v
}

// setOAuthStateCookie sets a CSRF state cookie when an OAuth authURL is returned.
// Extracted to avoid duplicating the cookie logic in multiple paths.
func setOAuthStateCookie(w http.ResponseWriter, r *http.Request, authURL string) {
	if authURL == "" {
		return
	}
	parsedURL, err := url.Parse(authURL)
	if err != nil {
		return
	}
	stateParam := parsedURL.Query().Get("state")
	if stateParam == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "mcp_oauth_state",
		Value:    stateParam,
		Path:     "/v1/mcp/oauth/callback",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
}

func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// headersSent checks if the response writer has already sent headers.
// This prevents writing error responses over streaming data.
// We use a custom header set by MCPProxy before writing the first byte.
func headersSent(w http.ResponseWriter) bool {
	return w.Header().Get("X-MCP-Proxy-Streamed") != ""
}

// writeJSONError sends a structured JSON error response.
// Matches agentstack's {\"detail\": \"...\"} pattern.
func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// mapServiceError translates typed service errors to (message, httpStatus) pairs.
func mapServiceError(err error) (string, int) {
	switch {
	case errors.Is(err, ErrConnectorNotFound):
		return "Connector not found", http.StatusNotFound
	case errors.Is(err, ErrDuplicateConnector):
		return err.Error(), http.StatusConflict
	case errors.Is(err, ErrConnectorLimitReached):
		return err.Error(), http.StatusForbidden
	case errors.Is(err, ErrInvalidConnectorState):
		return err.Error(), http.StatusConflict
	case errors.Is(err, ErrConnectorNotConnected):
		return "Connector is not connected", http.StatusConflict
	case errors.Is(err, ErrOAuthStateInvalid):
		return "Invalid or expired OAuth state", http.StatusBadRequest
	case errors.Is(err, ErrCircuitOpen):
		return "Remote server is temporarily unavailable", http.StatusServiceUnavailable
	case errors.Is(err, ErrConcurrencyLimit):
		return "Too many concurrent requests — try again later", http.StatusTooManyRequests
	case errors.Is(err, ErrUserConcurrencyLimit):
		return "Too many concurrent requests for your account — try again later", http.StatusTooManyRequests
	default:
		return "Internal server error", http.StatusInternalServerError
	}
}
