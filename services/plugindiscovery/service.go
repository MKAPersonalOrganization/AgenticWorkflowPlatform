package plugindiscovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/toolregistry"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Service orchestrates MCP connector lifecycle: create, connect, discover tools, OAuth.
type Service struct {
	log          *logger.Logger
	repo         domain.MCPConnectorRepository
	sessions     *SessionManager
	oauth        *OAuthService
	toolRegistry *toolregistry.Registry

	// oauthStates stores transient PKCE state during OAuth flows.
	oauthStatesMu sync.RWMutex
	oauthStates   map[string]*OAuthState // state param → OAuthState
}

// NewService creates a new MCP discovery service.
func NewService(log *logger.Logger, repo domain.MCPConnectorRepository, callbackURL string, toolRegistry *toolregistry.Registry) *Service {
	return &Service{
		log:          log,
		repo:         repo,
		sessions:     NewSessionManager(),
		oauth:        NewOAuthService(callbackURL),
		oauthStates:  make(map[string]*OAuthState),
		toolRegistry: toolRegistry,
	}
}

// CreateConnector registers a new MCP server URL for a user.
func (s *Service) CreateConnector(ctx context.Context, userID, serverURL, title string) (*domain.MCPConnector, error) {
	// Validate URL
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("server URL must use http or https scheme")
	}

	// Basic SSRF protection: block obvious internal addresses.
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return nil, fmt.Errorf("server URL cannot point to localhost")
	}
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "172.") {
		return nil, fmt.Errorf("server URL cannot point to private network addresses")
	}

	if title == "" {
		title = parsed.Host
	}

	connector := &domain.MCPConnector{
		UserID:    userID,
		Title:     title,
		ServerURL: serverURL,
		Status:    domain.ConnectorStatusCreated,
	}

	if err := s.repo.Create(ctx, connector); err != nil {
		return nil, fmt.Errorf("creating connector: %w", err)
	}

	s.log.Info(ctx, "mcp connector created", "id", connector.ID, "url", serverURL, "user_id", userID)
	return connector, nil
}

// ConnectConnector attempts to establish an MCP session and discover tools.
// If the server responds with 401, triggers the OAuth flow and returns the auth URL.
func (s *Service) ConnectConnector(ctx context.Context, connectorID string) (*domain.MCPConnector, string, error) {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return nil, "", fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, "", fmt.Errorf("connector not found: %s", connectorID)
	}

	// Update status to connecting.
	connector.Status = domain.ConnectorStatusConnecting
	connector.ErrorMsg = ""
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector status: %w", err)
	}

	// Build HTTP client (with OAuth token if available).
	httpClient := s.buildHTTPClient(connector)

	// Attempt MCP connection.
	session, err := s.sessions.Connect(ctx, connector.ID, connector.ServerURL, httpClient)
	if err != nil {
		// Check if this looks like an auth error (401).
		if isAuthError(err) {
			return s.initiateOAuth(ctx, connector)
		}

		// Connection failed.
		connector.Status = domain.ConnectorStatusError
		connector.ErrorMsg = err.Error()
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("connecting to MCP server: %w", err)
	}

	// Connection succeeded — discover tools.
	if err := s.discoverTools(ctx, connector, session); err != nil {
		s.log.Warn(ctx, "tool discovery failed", "connector_id", connectorID, "error", err)
		connector.ErrorMsg = "connected but tool discovery failed: " + err.Error()
	}

	// Update connector with server metadata.
	initResult := session.InitializeResult()
	if initResult != nil {
		connector.ServerName = toNullString(initResult.ServerInfo.Name)
		connector.ServerVersion = toNullString(initResult.ServerInfo.Version)
		capJSON, _ := json.Marshal(initResult.Capabilities)
		connector.Capabilities = toNullString(string(capJSON))
	}
	connector.Status = domain.ConnectorStatusConnected
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector after connect: %w", err)
	}

	// Register connector toolset + individual tools in the shared registry
	// so they appear in the plugins sidebar and can be toggled.
	s.registerConnectorInToolRegistry(ctx, connector)

	s.log.Info(ctx, "mcp connector connected", "id", connector.ID, "server", connector.ServerName.String)
	return connector, "", nil
}

// DisconnectConnector closes the session and updates status.
func (s *Service) DisconnectConnector(ctx context.Context, connectorID string) error {
	s.sessions.Disconnect(connectorID)

	// Remove the toolset from the plugin system — it only exists while connected.
	s.unregisterConnectorFromToolRegistry(ctx, connectorID)

	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return fmt.Errorf("connector not found: %s", connectorID)
	}

	connector.Status = domain.ConnectorStatusDisconnected
	if err := s.repo.Update(ctx, connector); err != nil {
		return fmt.Errorf("updating connector status: %w", err)
	}

	s.log.Info(ctx, "mcp connector disconnected", "id", connectorID)
	return nil
}

// DeleteConnector disconnects and removes a connector.
func (s *Service) DeleteConnector(ctx context.Context, connectorID, userID string) error {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return fmt.Errorf("connector not found: %s", connectorID)
	}
	if connector.UserID != userID {
		return fmt.Errorf("connector does not belong to user")
	}

	s.sessions.Disconnect(connectorID)

	// Remove the toolset and its tools from the shared registry.
	s.unregisterConnectorFromToolRegistry(ctx, connectorID)

	if err := s.repo.Delete(ctx, connectorID); err != nil {
		return fmt.Errorf("deleting connector: %w", err)
	}

	s.log.Info(ctx, "mcp connector deleted", "id", connectorID, "user_id", userID)
	return nil
}

// GetConnector retrieves a connector by ID.
func (s *Service) GetConnector(ctx context.Context, connectorID string) (*domain.MCPConnector, error) {
	return s.repo.GetByID(ctx, connectorID)
}

// ListConnectors returns all connectors for a user.
func (s *Service) ListConnectors(ctx context.Context, userID string) ([]domain.MCPConnector, error) {
	connectors, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Sync live status: if we have an active session but DB says disconnected, update.
	for i := range connectors {
		if s.sessions.IsConnected(connectors[i].ID) && connectors[i].Status != domain.ConnectorStatusConnected {
			connectors[i].Status = domain.ConnectorStatusConnected
		} else if !s.sessions.IsConnected(connectors[i].ID) && connectors[i].Status == domain.ConnectorStatusConnected {
			connectors[i].Status = domain.ConnectorStatusDisconnected
		}
	}
	return connectors, nil
}

// GetConnectorTools returns the discovered tools for a connector.
func (s *Service) GetConnectorTools(ctx context.Context, connectorID string) ([]domain.MCPDiscoveredTool, error) {
	return s.repo.ListTools(ctx, connectorID)
}

// RefreshTools re-discovers tools from a connected MCP server.
func (s *Service) RefreshTools(ctx context.Context, connectorID string) ([]domain.MCPDiscoveredTool, error) {
	session := s.sessions.GetSession(connectorID)
	if session == nil {
		return nil, fmt.Errorf("connector is not connected: %s", connectorID)
	}

	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, fmt.Errorf("connector not found: %s", connectorID)
	}

	if err := s.discoverTools(ctx, connector, session); err != nil {
		return nil, fmt.Errorf("refreshing tools: %w", err)
	}

	return s.repo.ListTools(ctx, connectorID)
}

// HandleOAuthCallback processes the OAuth redirect callback.
func (s *Service) HandleOAuthCallback(ctx context.Context, stateParam, code string) (*domain.MCPConnector, error) {
	// Look up the OAuth state.
	s.oauthStatesMu.Lock()
	oauthState, ok := s.oauthStates[stateParam]
	if ok {
		delete(s.oauthStates, stateParam)
	}
	s.oauthStatesMu.Unlock()

	if !ok {
		return nil, fmt.Errorf("invalid or expired OAuth state")
	}

	connector, err := s.repo.GetByID(ctx, oauthState.ConnectorID)
	if err != nil {
		return nil, fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, fmt.Errorf("connector not found")
	}

	// Retrieve cached auth server metadata.
	var authMeta OAuthMetadata
	if connector.OAuthServerMeta.Valid {
		if err := json.Unmarshal([]byte(connector.OAuthServerMeta.String), &authMeta); err != nil {
			return nil, fmt.Errorf("decoding cached auth metadata: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no cached auth metadata for connector")
	}

	clientID := ""
	if connector.OAuthClientID.Valid {
		clientID = connector.OAuthClientID.String
	}
	clientSecret := ""
	if connector.OAuthClientSecret.Valid {
		clientSecret = connector.OAuthClientSecret.String
	}

	// Exchange authorization code for tokens.
	tokenResp, err := s.oauth.ExchangeCode(ctx, &authMeta, clientID, clientSecret, code, oauthState.CodeVerifier)
	if err != nil {
		connector.Status = domain.ConnectorStatusError
		connector.ErrorMsg = "token exchange failed: " + err.Error()
		_ = s.repo.Update(ctx, connector)
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	// Store tokens.
	tokenJSON, _ := json.Marshal(tokenResp)
	connector.OAuthTokenJSON = toNullString(string(tokenJSON))
	connector.Status = domain.ConnectorStatusConnecting
	connector.ErrorMsg = ""
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, fmt.Errorf("saving tokens: %w", err)
	}

	s.log.Info(ctx, "oauth tokens acquired", "connector_id", connector.ID)

	// Reconnect with tokens.
	_, _, err = s.ConnectConnector(ctx, connector.ID)
	if err != nil {
		return nil, fmt.Errorf("reconnecting after OAuth: %w", err)
	}

	// Re-fetch to get final status.
	return s.repo.GetByID(ctx, connector.ID)
}

// CallTool executes a tool on a remote MCP server via the live session.
func (s *Service) CallTool(ctx context.Context, connectorID, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	session := s.sessions.GetSession(connectorID)
	if session == nil {
		return nil, fmt.Errorf("connector is not connected: %s", connectorID)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("calling tool %s: %w", toolName, err)
	}
	return result, nil
}

// GetSessionManager returns the session manager for direct access (e.g. by tool resolver).
func (s *Service) GetSessionManager() *SessionManager {
	return s.sessions
}

// Shutdown gracefully closes all sessions.
func (s *Service) Shutdown() {
	s.sessions.CloseAll()
}

// ResolveEnabledTools returns MCP executable tools for a user, filtered by the provided
// permission checker callbacks. This allows the agent orchestrator to inject only
// the MCP tools that the user has explicitly enabled for a given agent.
func (s *Service) ResolveEnabledTools(
	ctx context.Context,
	userID string,
	isToolSetEnabled func(toolSetID string) bool,
	isToolEnabled func(toolSetID, toolID string) bool,
) []agent.ExecutableTool {
	connectors, err := s.ListConnectors(ctx, userID)
	if err != nil {
		return nil
	}

	var tools []agent.ExecutableTool
	for _, conn := range connectors {
		if conn.Status != domain.ConnectorStatusConnected {
			continue
		}

		toolSetID := "mcp-" + conn.ID
		if !isToolSetEnabled(toolSetID) {
			continue
		}

		session := s.sessions.GetSession(conn.ID)
		if session == nil {
			continue
		}

		discoveredTools, err := s.GetConnectorTools(ctx, conn.ID)
		if err != nil {
			continue
		}

		for _, dt := range discoveredTools {
			toolID := "mcp-" + conn.ID + "-" + dt.Name
			if !isToolEnabled(toolSetID, toolID) {
				continue
			}
			tools = append(tools, NewMCPRemoteTool(conn.ID, dt.Name, dt.Description, dt.InputSchema, session))
		}
	}
	return tools
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// initiateOAuth starts the OAuth flow for a connector that returned 401.
func (s *Service) initiateOAuth(ctx context.Context, connector *domain.MCPConnector) (*domain.MCPConnector, string, error) {
	s.log.Info(ctx, "initiating OAuth flow", "connector_id", connector.ID, "url", connector.ServerURL)

	// Step 1: Discover protected resource metadata (RFC 9728).
	resourceMeta, err := s.oauth.DiscoverProtectedResourceMetadata(ctx, connector.ServerURL)
	if err != nil {
		connector.Status = domain.ConnectorStatusError
		connector.ErrorMsg = "failed to discover OAuth metadata: " + err.Error()
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("discovering resource metadata: %w", err)
	}

	if len(resourceMeta.AuthorizationServers) == 0 {
		connector.Status = domain.ConnectorStatusError
		connector.ErrorMsg = "no authorization servers found in metadata"
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("no authorization servers in resource metadata")
	}

	// Step 2: Discover auth server metadata (RFC 8414).
	authServerURL := resourceMeta.AuthorizationServers[0]
	authMeta, err := s.oauth.DiscoverAuthServerMetadata(ctx, authServerURL)
	if err != nil {
		connector.Status = domain.ConnectorStatusError
		connector.ErrorMsg = "failed to discover auth server: " + err.Error()
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("discovering auth server metadata: %w", err)
	}

	// Cache auth server metadata on the connector.
	metaJSON, _ := json.Marshal(authMeta)
	connector.OAuthServerMeta = toNullString(string(metaJSON))

	// Step 3: Dynamic Client Registration (if needed and supported).
	clientID := ""
	clientSecret := ""
	if connector.OAuthClientID.Valid && connector.OAuthClientID.String != "" {
		clientID = connector.OAuthClientID.String
		if connector.OAuthClientSecret.Valid {
			clientSecret = connector.OAuthClientSecret.String
		}
	} else if authMeta.RegistrationEndpoint != "" {
		cid, cs, err := s.oauth.RegisterClient(ctx, authMeta.RegistrationEndpoint, connector.Title)
		if err != nil {
			s.log.Warn(ctx, "dynamic client registration failed", "error", err)
			connector.Status = domain.ConnectorStatusError
			connector.ErrorMsg = "dynamic client registration failed: " + err.Error()
			_ = s.repo.Update(ctx, connector)
			return connector, "", fmt.Errorf("registering client: %w", err)
		}
		clientID = cid
		clientSecret = cs
		connector.OAuthClientID = toNullString(clientID)
		connector.OAuthClientSecret = toNullString(clientSecret)
	} else {
		connector.Status = domain.ConnectorStatusError
		connector.ErrorMsg = "OAuth required but no client credentials and no registration endpoint"
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("no OAuth client credentials and no registration endpoint")
	}

	// Step 4: Generate PKCE and build authorization URL.
	state := GenerateState()
	codeVerifier := GenerateCodeVerifier()
	challenge := CodeChallenge(codeVerifier)

	scopes := resourceMeta.ScopesSupported
	if len(scopes) == 0 {
		scopes = authMeta.ScopesSupported
	}

	authURL := s.oauth.BuildAuthorizationURL(authMeta, clientID, state, challenge, scopes)

	// Store transient OAuth state.
	s.oauthStatesMu.Lock()
	s.oauthStates[state] = &OAuthState{
		ConnectorID:  connector.ID,
		State:        state,
		CodeVerifier: codeVerifier,
	}
	s.oauthStatesMu.Unlock()

	// Schedule cleanup of stale state after 10 minutes.
	go func() {
		time.Sleep(10 * time.Minute)
		s.oauthStatesMu.Lock()
		delete(s.oauthStates, state)
		s.oauthStatesMu.Unlock()
	}()

	// Update connector status.
	connector.Status = domain.ConnectorStatusAuthRequired
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector for OAuth: %w", err)
	}

	s.log.Info(ctx, "oauth authorization URL generated", "connector_id", connector.ID)
	return connector, authURL, nil
}

// discoverTools calls ListTools on the MCP session and persists the results.
// unregisterConnectorFromToolRegistry removes the connector's toolset and
// tools from the shared tool registry (DB + cache).
func (s *Service) unregisterConnectorFromToolRegistry(ctx context.Context, connectorID string) {
	if s.toolRegistry == nil {
		return
	}

	toolSetID := "mcp-" + connectorID
	if err := s.toolRegistry.UnregisterToolSet(ctx, toolSetID); err != nil {
		s.log.Warn(ctx, "failed to unregister mcp toolset", "tool_set_id", toolSetID, "error", err)
	} else {
		s.log.Info(ctx, "mcp connector unregistered from tool registry", "tool_set_id", toolSetID)
	}
}

// registerConnectorInToolRegistry registers the connector's toolset and
// individual tools into the shared tool registry so they show up in the
// plugins sidebar and can be toggled per-agent like built-in toolsets.
func (s *Service) registerConnectorInToolRegistry(ctx context.Context, connector *domain.MCPConnector) {
	if s.toolRegistry == nil {
		return
	}

	toolSetID := "mcp-" + connector.ID
	name := connector.Title
	if connector.ServerName.Valid && connector.ServerName.String != "" {
		name = connector.ServerName.String
	}

	discoveredTools, err := s.repo.ListTools(ctx, connector.ID)
	if err != nil {
		s.log.Warn(ctx, "failed to list tools for registry registration", "connector_id", connector.ID, "error", err)
		return
	}

	// Build the tool ID list first.
	toolIDs := make([]string, 0, len(discoveredTools))
	for _, dt := range discoveredTools {
		toolIDs = append(toolIDs, "mcp-"+connector.ID+"-"+dt.Name)
	}

	// Register the toolset BEFORE individual tools because the tools table
	// has a FK constraint (tool_set_id → tool_sets.id).
	if err := s.toolRegistry.RegisterToolSet(ctx, &toolregistry.ToolSet{
		ID:          toolSetID,
		Name:        name,
		Description: "MCP Server: " + connector.ServerURL,
		Category:    "MCP",
		ToolIDs:     toolIDs,
	}); err != nil {
		s.log.Warn(ctx, "failed to register mcp toolset", "tool_set_id", toolSetID, "error", err)
		return
	}

	// Now register the individual tools (FK parent row exists).
	for _, dt := range discoveredTools {
		toolID := "mcp-" + connector.ID + "-" + dt.Name
		if err := s.toolRegistry.RegisterTool(ctx, &toolregistry.Tool{
			ID:          toolID,
			Name:        dt.Name,
			Description: dt.Description,
			Category:    "MCP",
			ToolSetID:   toolSetID,
		}); err != nil {
			s.log.Warn(ctx, "failed to register mcp tool", "tool_id", toolID, "error", err)
		}
	}

	s.log.Info(ctx, "mcp connector registered in tool registry", "tool_set_id", toolSetID, "tool_count", len(toolIDs))
}

func (s *Service) discoverTools(ctx context.Context, connector *domain.MCPConnector, session *mcp.ClientSession) error {
	var tools []domain.MCPDiscoveredTool

	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return fmt.Errorf("iterating tools: %w", err)
		}

		schemaJSON := "{}"
		if tool.InputSchema != nil {
			b, _ := json.Marshal(tool.InputSchema)
			schemaJSON = string(b)
		}

		annotationsJSON := "{}"
		if tool.Annotations != nil {
			b, _ := json.Marshal(tool.Annotations)
			annotationsJSON = string(b)
		}

		tools = append(tools, domain.MCPDiscoveredTool{
			ConnectorID: connector.ID,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaJSON,
			Annotations: annotationsJSON,
		})
	}

	if err := s.repo.UpsertTools(ctx, connector.ID, tools); err != nil {
		return fmt.Errorf("persisting discovered tools: %w", err)
	}

	s.log.Info(ctx, "tools discovered", "connector_id", connector.ID, "count", len(tools))
	return nil
}

// buildHTTPClient creates an HTTP client with OAuth token injection if available.
func (s *Service) buildHTTPClient(connector *domain.MCPConnector) *http.Client {
	if !connector.OAuthTokenJSON.Valid || connector.OAuthTokenJSON.String == "" {
		return nil // Use default
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal([]byte(connector.OAuthTokenJSON.String), &tokenResp); err != nil {
		return nil
	}

	if tokenResp.AccessToken == "" {
		return nil
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &BearerTokenTransport{
			Token: tokenResp.AccessToken,
		},
	}
}

// isAuthError checks if an error looks like a 401 Unauthorized.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") || strings.Contains(msg, "unauthorized")
}

// toNullString converts a string to sql.NullString.
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
