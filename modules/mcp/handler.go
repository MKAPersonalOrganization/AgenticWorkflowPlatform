package mcp

import (
	"aicompass/foundation/i18n"
	"aicompass/foundation/logger"
	"aicompass/foundation/auth"
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

// requireMCPUser is the MCP-package variant of the auth prologue. It
// returns the authenticated user or writes a translated 401 and returns
// nil so the caller returns immediately.
func requireMCPUser(w http.ResponseWriter, r *http.Request) *auth.User {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return nil
	}
	return user
}

// respondConnectorJSON writes a connector response as JSON with the
// given optional status. Consolidates the Header/WriteHeader/Encode
// pattern that every handler repeats.
func respondConnectorJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 {
		w.WriteHeader(status)
	}
	_ = json.NewEncoder(w).Encode(body)
}

// toolsResponseList marshals the per-tool JSON envelope shared by
// listTools and refreshTools. Unmarshalling the schema / annotations
// columns here keeps the handlers as one-liners.
func toolsResponseList(tools []MCPDiscoveredTool) []toolResponse {
	result := make([]toolResponse, 0, len(tools))
	for _, t := range tools {
		var schema, annotations any
		_ = json.Unmarshal([]byte(t.InputSchema), &schema)
		_ = json.Unmarshal([]byte(t.Annotations), &annotations)
		result = append(result, toolResponse{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
			Annotations: annotations,
		})
	}
	return result
}

// connectorResponseWithTools builds a connector response with its
// currently-known tool count. Swallows tool-fetch errors the way every
// caller does — a nil-safe len(tools) is plenty for the UI.
func (h *handler) connectorResponseWithTools(r *http.Request, connector *MCPConnector, userID string) connectorResponse {
	tools, _ := h.service.GetConnectorTools(r.Context(), connector.ID, userID)
	return toConnectorResponse(*connector, len(tools))
}

// listConnectors handles GET /v1/mcp/connectors
func (h *handler) listConnectors(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
		return
	}
	connectors, err := h.service.ListConnectors(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list connectors", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to list MCP connectors"), http.StatusInternalServerError)
		return
	}
	toolCounts, _ := h.service.CountToolsByConnectors(r.Context(), connectorIDs(connectors))
	result := make([]connectorResponse, 0, len(connectors))
	for _, c := range connectors {
		result = append(result, toConnectorResponse(c, toolCounts[c.ID]))
	}
	respondConnectorJSON(w, 0, map[string]any{"connectors": result})
}

// connectorIDs projects a connector slice to the id slice the service's
// batch CountToolsByConnectors call expects — avoids the N+1 per-row
// count query.
func connectorIDs(connectors []MCPConnector) []string {
	ids := make([]string, len(connectors))
	for i, c := range connectors {
		ids[i] = c.ID
	}
	return ids
}

// createConnector handles POST /v1/mcp/connectors
func (h *handler) createConnector(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
		return
	}
	req, ok := decodeCreateConnectorRequest(w, r)
	if !ok {
		return
	}
	connector, err := h.service.CreateConnector(r.Context(), user.ID, req.ServerURL, req.Title, req.OAuthClientID, req.OAuthClientSecret)
	if err != nil {
		h.log.Error(r.Context(), "failed to create connector", "error", err)
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}
	respondConnectorJSON(w, http.StatusCreated, toConnectorResponse(*connector, 0))
}

// decodeCreateConnectorRequest reads + validates the create-connector
// body (1 MiB cap, required ServerURL). On any failure it writes a
// translated 400 and returns ok=false.
func decodeCreateConnectorRequest(w http.ResponseWriter, r *http.Request) (*createConnectorRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return nil, false
	}
	if req.ServerURL == "" {
		http.Error(w, i18n.T(r.Context(), "Server URL is required"), http.StatusBadRequest)
		return nil, false
	}
	return &req, true
}

// getConnector handles GET /v1/mcp/connectors/{id}
func (h *handler) getConnector(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
		return
	}
	id := chi.URLParam(r, "id")
	connector, err := h.service.GetConnector(r.Context(), id, user.ID)
	if err != nil {
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}
	respondConnectorJSON(w, 0, h.connectorResponseWithTools(r, connector, user.ID))
}

// deleteConnector handles DELETE /v1/mcp/connectors/{id}
func (h *handler) deleteConnector(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
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
	user := requireMCPUser(w, r)
	if user == nil {
		return
	}
	id := chi.URLParam(r, "id")
	accessToken := readOptionalAccessToken(r)

	updated, authURL, err := h.service.ConnectConnector(r.Context(), id, user.ID, accessToken)
	if err != nil {
		h.writeConnectFailure(w, r, id, user.ID, updated, authURL, err)
		return
	}
	setOAuthStateCookie(w, r, authURL)
	respondConnectorJSON(w, 0, connectResponse{
		Connector: ptr(h.connectorResponseWithTools(r, updated, user.ID)),
		AuthURL:   authURL,
	})
}

// readOptionalAccessToken best-effort decodes the agentstack
// access_token body (4 KiB cap). Missing / malformed body is silent
// because the field is optional.
func readOptionalAccessToken(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
	return body.AccessToken
}

// writeConnectFailure keeps `connect` flat by handling the
// "return the partial connector state even on error" branch: when the
// service surfaced an updated connector, we still emit a 422 with the
// same shape so the UI can render the status; otherwise we fall back
// to the normal service-error mapping.
func (h *handler) writeConnectFailure(w http.ResponseWriter, r *http.Request, id, userID string, updated *MCPConnector, authURL string, err error) {
	h.log.Error(r.Context(), "failed to connect", "error", err, "connector_id", id)
	if updated == nil {
		msg, code := mapServiceError(err)
		writeJSONError(w, msg, code)
		return
	}
	setOAuthStateCookie(w, r, authURL)
	respondConnectorJSON(w, http.StatusUnprocessableEntity, connectResponse{
		Connector: ptr(h.connectorResponseWithTools(r, updated, userID)),
		AuthURL:   authURL,
	})
}

// disconnect handles POST /v1/mcp/connectors/{id}/disconnect
func (h *handler) disconnect(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
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
	respondConnectorJSON(w, 0, h.connectorResponseWithTools(r, updated, user.ID))
}

// listTools handles GET /v1/mcp/connectors/{id}/tools
func (h *handler) listTools(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
		return
	}
	id := chi.URLParam(r, "id")
	tools, err := h.service.GetConnectorTools(r.Context(), id, user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list tools", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to list tools"), http.StatusInternalServerError)
		return
	}
	respondConnectorJSON(w, 0, map[string]any{"tools": toolsResponseList(tools)})
}

// refreshTools handles POST /v1/mcp/connectors/{id}/refresh
func (h *handler) refreshTools(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
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
	respondConnectorJSON(w, 0, map[string]any{"tools": toolsResponseList(tools)})
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
	respondConnectorJSON(w, 0, map[string]any{"presets": result})
}

// mcpProxy handles all methods on /v1/mcp/connectors/{id}/mcp/*
// Acts as an authenticated reverse proxy to the connector's MCP server.
// This is agentstack's core pattern: the platform forwards raw MCP protocol
// messages so external clients can use connectors directly.
func (h *handler) mcpProxy(w http.ResponseWriter, r *http.Request) {
	user := requireMCPUser(w, r)
	if user == nil {
		return
	}
	id := chi.URLParam(r, "id")
	err := h.service.MCPProxy(r.Context(), id, user.ID, w, r)
	if err == nil {
		return
	}
	h.log.Error(r.Context(), "mcp proxy error", "error", err, "connector_id", id)
	if headersSent(w) {
		return // streaming already started — can't send a structured error
	}
	msg, code := mapServiceError(err)
	writeJSONError(w, msg, code)
}

// oauthCallback handles GET /v1/mcp/oauth/callback
func (h *handler) oauthCallback(w http.ResponseWriter, r *http.Request) {
	if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
		h.handleOAuthProviderError(w, r, oauthErr, r.URL.Query().Get("error_description"))
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
		writeOAuthFailureHTML(w, err.Error())
		return
	}
	writeOAuthSuccessHTML(w, connector)
}

// handleOAuthProviderError renders the RFC 6749 §4.1.2.1 error-redirect
// page, choosing the authz-server's error_description over the bare
// error code when both are present.
func (h *handler) handleOAuthProviderError(w http.ResponseWriter, r *http.Request, oauthErr, desc string) {
	if desc == "" {
		desc = oauthErr
	}
	h.log.Error(r.Context(), "OAuth provider returned error", "error", oauthErr, "description", desc)
	writeOAuthFailureHTML(w, desc)
}

// writeOAuthFailureHTML posts a `{success:false, error:…}` message back
// to the opener window and closes the popup.
func writeOAuthFailureHTML(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><script>
window.opener && window.opener.postMessage({type:'mcp-oauth-callback',success:false,error:` + jsonEscape(reason) + `},window.location.origin);
window.close();
</script><p>Authentication failed: ` + reason + `. You can close this window.</p></body></html>`))
}

// writeOAuthSuccessHTML posts a `{success:true, connector_id, status}`
// message back to the opener and closes the popup.
func writeOAuthSuccessHTML(w http.ResponseWriter, connector *MCPConnector) {
	connectorJSON, _ := json.Marshal(map[string]any{
		"type":         "mcp-oauth-callback",
		"success":      true,
		"connector_id": connector.ID,
		"status":       string(connector.Status),
	})
	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><script>
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
