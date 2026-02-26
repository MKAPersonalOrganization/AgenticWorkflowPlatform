package plugindiscovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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

// --------------------------------------------------------------------------
// Configuration constants
// --------------------------------------------------------------------------

const (
	// maxConnectorsPerUser limits the number of MCP connectors a single user
	// can register. This prevents unbounded resource consumption.
	maxConnectorsPerUser = 20

	// probeMaxRetries is the number of times probeConnector will attempt the
	// MCP handshake before giving up.
	probeMaxRetries = 3

	// probeRetryDelay is the initial back-off between probe retries.
	probeRetryDelay = 2 * time.Second

	// oauthStateExpiry controls how long a pending OAuth state is kept before
	// being garbage-collected.
	oauthStateExpiry = 10 * time.Minute

	// healthCheckInterval is the recommended ticker duration for
	// RefreshAllConnectors (caller owns the ticker).
	healthCheckInterval = 30 * time.Second
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

// --------------------------------------------------------------------------
// CRUD
// --------------------------------------------------------------------------

// CreateConnector registers a new MCP server URL for a user.
func (s *Service) CreateConnector(ctx context.Context, userID, serverURL, title string) (*domain.MCPConnector, error) {
	// Validate URL.
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("server URL must use http or https scheme")
	}

	// SSRF protection: block obvious internal addresses.
	if err := validateExternalHost(parsed.Hostname()); err != nil {
		return nil, err
	}

	// Enforce per-user connector limit.
	count, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("checking connector count: %w", err)
	}
	if count >= maxConnectorsPerUser {
		return nil, fmt.Errorf("maximum of %d MCP connectors per user reached", maxConnectorsPerUser)
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

// GetConnector retrieves a connector by ID.
func (s *Service) GetConnector(ctx context.Context, connectorID string) (*domain.MCPConnector, error) {
	return s.repo.GetByID(ctx, connectorID)
}

// ListConnectors returns all connectors for a user, syncing in-memory
// session liveness with persisted status.
func (s *Service) ListConnectors(ctx context.Context, userID string) ([]domain.MCPConnector, error) {
	connectors, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
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

// --------------------------------------------------------------------------
// Connection lifecycle
// --------------------------------------------------------------------------

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

	// Transition to connecting.
	connector.Status = domain.ConnectorStatusConnecting
	connector.ErrorMsg = ""
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector status: %w", err)
	}

	// Build HTTP client (with OAuth token if available).
	httpClient := s.buildHTTPClient(connector)

	// Probe: attempt MCP handshake with retries.
	if err := s.probeConnector(ctx, connector, httpClient); err != nil {
		return s.handleConnectionError(ctx, connector, err)
	}

	// Probe succeeded — open the persistent session.
	session, err := s.sessions.Connect(ctx, connector.ID, connector.ServerURL, httpClient)
	if err != nil {
		connector.ErrorMsg = err.Error()
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("establishing persistent session: %w", err)
	}

	// Discover tools.
	if err := s.discoverTools(ctx, connector, session); err != nil {
		s.log.Warn(ctx, "tool discovery failed", "connector_id", connectorID, "error", err)
		connector.ErrorMsg = "connected but tool discovery failed: " + err.Error()
	}

	// Capture server metadata from the initialization result.
	initResult := session.InitializeResult()
	if initResult != nil {
		connector.ServerName = toNullString(initResult.ServerInfo.Name)
		connector.ServerVersion = toNullString(initResult.ServerInfo.Version)
		capJSON, _ := json.Marshal(initResult.Capabilities)
		connector.Capabilities = toNullString(string(capJSON))
	}

	_ = connector.Transition(domain.ConnectorStatusConnected, "", false)
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector after connect: %w", err)
	}

	// Register in the plugin system so it appears in the sidebar.
	s.registerConnectorInToolRegistry(ctx, connector)

	s.log.Info(ctx, "mcp connector connected", "id", connector.ID, "server", connector.ServerName.String)
	return connector, "", nil
}

// DisconnectConnector closes the session, revokes tokens, and updates status.
func (s *Service) DisconnectConnector(ctx context.Context, connectorID string) error {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return fmt.Errorf("connector not found: %s", connectorID)
	}

	// Close the live session.
	s.sessions.Disconnect(connectorID)

	// Remove from plugin system.
	s.unregisterConnectorFromToolRegistry(ctx, connectorID)

	// Revoke OAuth token if present.
	s.revokeToken(ctx, connector)

	_ = connector.Transition(domain.ConnectorStatusDisconnected, "client request", true)
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
	s.unregisterConnectorFromToolRegistry(ctx, connectorID)
	s.revokeToken(ctx, connector)

	if err := s.repo.Delete(ctx, connectorID); err != nil {
		return fmt.Errorf("deleting connector: %w", err)
	}

	s.log.Info(ctx, "mcp connector deleted", "id", connectorID, "user_id", userID)
	return nil
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

// --------------------------------------------------------------------------
// Background health check
// --------------------------------------------------------------------------

// RefreshConnector probes a single connector and updates its status.
// Connected connectors that fail the probe are transitioned to disconnected.
// Transient disconnects (disconnect_permanent=false) are retried.
func (s *Service) RefreshConnector(ctx context.Context, connectorID string) {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil || connector == nil {
		return
	}
	if !connector.Refreshable() {
		return
	}

	httpClient := s.buildHTTPClient(connector)

	if err := s.probeConnector(ctx, connector, httpClient); err != nil {
		// Classify the error to decide permanence.
		permanent := isHTTPClientError(err) // 4xx errors are permanent
		reason := err.Error()

		// If it's a 401, revoke the token and mark permanent.
		if isAuthError(err) {
			s.revokeToken(ctx, connector)
			permanent = true
		}

		s.sessions.Disconnect(connectorID)
		s.unregisterConnectorFromToolRegistry(ctx, connectorID)
		_ = connector.Transition(domain.ConnectorStatusDisconnected, reason, permanent)
	} else {
		// Probe succeeded — make sure we have a live session.
		if !s.sessions.IsConnected(connectorID) {
			session, connErr := s.sessions.Connect(ctx, connectorID, connector.ServerURL, httpClient)
			if connErr != nil {
				_ = connector.Transition(domain.ConnectorStatusDisconnected, connErr.Error(), false)
			} else {
				// Re-discover tools on reconnect.
				_ = s.discoverTools(ctx, connector, session)
				_ = connector.Transition(domain.ConnectorStatusConnected, "", false)
				s.registerConnectorInToolRegistry(ctx, connector)
			}
		} else {
			_ = connector.Transition(domain.ConnectorStatusConnected, "", false)
		}
	}

	_ = s.repo.Update(ctx, connector)
}

// RefreshAllConnectors iterates all connectors and refreshes those that
// are eligible. Designed to be called from a background goroutine on a ticker.
func (s *Service) RefreshAllConnectors(ctx context.Context) {
	connectors, err := s.listAllRefreshable(ctx)
	if err != nil {
		s.log.Warn(ctx, "failed to list connectors for refresh", "error", err)
		return
	}
	for _, c := range connectors {
		s.RefreshConnector(ctx, c.ID)
	}
}

// StartHealthCheck launches a background goroutine that periodically
// probes all refreshable connectors. It stops when ctx is cancelled.
func (s *Service) StartHealthCheck(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(healthCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RefreshAllConnectors(ctx)
			}
		}
	}()
}

// --------------------------------------------------------------------------
// OAuth flow
// --------------------------------------------------------------------------

// HandleOAuthCallback processes the OAuth redirect callback.
func (s *Service) HandleOAuthCallback(ctx context.Context, stateParam, code string) (*domain.MCPConnector, error) {
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
		connector.ErrorMsg = "token exchange failed: " + err.Error()
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
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

	return s.repo.GetByID(ctx, connector.ID)
}

// --------------------------------------------------------------------------
// Tool execution
// --------------------------------------------------------------------------

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
// Probe & connection error handling
// --------------------------------------------------------------------------

// probeConnector performs a lightweight MCP handshake to verify that the server
// is reachable and the credentials are valid. It retries on transient errors.
func (s *Service) probeConnector(ctx context.Context, connector *domain.MCPConnector, httpClient *http.Client) error {
	var lastErr error

	for attempt := range probeMaxRetries {
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		transport := &mcp.StreamableClientTransport{
			Endpoint: connector.ServerURL,
		}
		if httpClient != nil {
			transport.HTTPClient = httpClient
		}

		session, err := s.sessions.client.Connect(probeCtx, transport, nil)
		if err == nil {
			// Probe succeeded — close the temporary session.
			session.Close()
			cancel()
			return nil
		}
		cancel()
		lastErr = err

		// Only retry on transient network errors.
		if !isTransientError(err) {
			return lastErr
		}

		if attempt < probeMaxRetries-1 {
			s.log.Warn(ctx, "probe attempt failed, retrying",
				"connector_id", connector.ID,
				"attempt", attempt+1,
				"error", err,
			)
			time.Sleep(probeRetryDelay * time.Duration(attempt+1))
		}
	}

	return lastErr
}

// handleConnectionError classifies the connection failure and returns the
// appropriate connector state and optional auth URL.
func (s *Service) handleConnectionError(ctx context.Context, connector *domain.MCPConnector, err error) (*domain.MCPConnector, string, error) {
	if isAuthError(err) {
		return s.initiateOAuth(ctx, connector)
	}

	connector.ErrorMsg = err.Error()

	if isTransientError(err) {
		// Transient: mark disconnected but allow refresh to retry.
		_ = connector.Transition(domain.ConnectorStatusDisconnected, err.Error(), false)
	} else {
		// Permanent: HTTP 4xx, SSRF, bad URL, etc.
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
	}

	_ = s.repo.Update(ctx, connector)
	return connector, "", fmt.Errorf("connecting to MCP server: %w", err)
}

// --------------------------------------------------------------------------
// OAuth initiation
// --------------------------------------------------------------------------

func (s *Service) initiateOAuth(ctx context.Context, connector *domain.MCPConnector) (*domain.MCPConnector, string, error) {
	s.log.Info(ctx, "initiating OAuth flow", "connector_id", connector.ID, "url", connector.ServerURL)

	resourceMeta, err := s.oauth.DiscoverProtectedResourceMetadata(ctx, connector.ServerURL)
	if err != nil {
		connector.ErrorMsg = "failed to discover OAuth metadata: " + err.Error()
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("discovering resource metadata: %w", err)
	}

	if len(resourceMeta.AuthorizationServers) == 0 {
		connector.ErrorMsg = "no authorization servers found in metadata"
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("no authorization servers in resource metadata")
	}

	authServerURL := resourceMeta.AuthorizationServers[0]
	authMeta, err := s.oauth.DiscoverAuthServerMetadata(ctx, authServerURL)
	if err != nil {
		connector.ErrorMsg = "failed to discover auth server: " + err.Error()
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("discovering auth server metadata: %w", err)
	}

	metaJSON, _ := json.Marshal(authMeta)
	connector.OAuthServerMeta = toNullString(string(metaJSON))

	clientID := ""
	clientSecret := ""
	if connector.OAuthClientID.Valid && connector.OAuthClientID.String != "" {
		clientID = connector.OAuthClientID.String
		if connector.OAuthClientSecret.Valid {
			clientSecret = connector.OAuthClientSecret.String
		}
	} else if authMeta.RegistrationEndpoint != "" {
		cid, cs, regErr := s.oauth.RegisterClient(ctx, authMeta.RegistrationEndpoint, connector.Title)
		if regErr != nil {
			s.log.Warn(ctx, "dynamic client registration failed", "error", regErr)
			connector.ErrorMsg = "dynamic client registration failed: " + regErr.Error()
			_ = connector.Transition(domain.ConnectorStatusError, "", false)
			_ = s.repo.Update(ctx, connector)
			return connector, "", fmt.Errorf("registering client: %w", regErr)
		}
		clientID = cid
		clientSecret = cs
		connector.OAuthClientID = toNullString(clientID)
		connector.OAuthClientSecret = toNullString(clientSecret)
	} else {
		connector.ErrorMsg = "OAuth required but no client credentials and no registration endpoint"
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
		_ = s.repo.Update(ctx, connector)
		return connector, "", fmt.Errorf("no OAuth client credentials and no registration endpoint")
	}

	state := GenerateState()
	codeVerifier := GenerateCodeVerifier()
	challenge := CodeChallenge(codeVerifier)

	scopes := resourceMeta.ScopesSupported
	if len(scopes) == 0 {
		scopes = authMeta.ScopesSupported
	}

	authURL := s.oauth.BuildAuthorizationURL(authMeta, clientID, state, challenge, scopes)

	s.oauthStatesMu.Lock()
	s.oauthStates[state] = &OAuthState{
		ConnectorID:  connector.ID,
		State:        state,
		CodeVerifier: codeVerifier,
	}
	s.oauthStatesMu.Unlock()

	// Garbage-collect stale state after expiry.
	go func() {
		time.Sleep(oauthStateExpiry)
		s.oauthStatesMu.Lock()
		delete(s.oauthStates, state)
		s.oauthStatesMu.Unlock()
	}()

	_ = connector.Transition(domain.ConnectorStatusAuthRequired, "", false)
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector for OAuth: %w", err)
	}

	s.log.Info(ctx, "oauth authorization URL generated", "connector_id", connector.ID)
	return connector, authURL, nil
}

// --------------------------------------------------------------------------
// Token revocation
// --------------------------------------------------------------------------

// revokeToken attempts to revoke the OAuth access token at the authorization
// server's revocation endpoint. Failures are logged but not propagated — the
// token is cleared locally regardless.
func (s *Service) revokeToken(ctx context.Context, connector *domain.MCPConnector) {
	if !connector.OAuthTokenJSON.Valid || connector.OAuthTokenJSON.String == "" {
		return
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal([]byte(connector.OAuthTokenJSON.String), &tokenResp); err != nil {
		return
	}
	if tokenResp.AccessToken == "" {
		return
	}

	// Try to revoke remotely if we have auth server metadata with a revocation endpoint.
	if connector.OAuthServerMeta.Valid {
		var authMeta OAuthMetadata
		if err := json.Unmarshal([]byte(connector.OAuthServerMeta.String), &authMeta); err == nil {
			if authMeta.RevocationEndpoint != "" {
				if err := s.oauth.RevokeToken(ctx, &authMeta, tokenResp.AccessToken); err != nil {
					s.log.Warn(ctx, "token revocation failed", "connector_id", connector.ID, "error", err)
				} else {
					s.log.Info(ctx, "token revoked", "connector_id", connector.ID)
				}
			}
		}
	}

	// Clear locally regardless of remote revocation outcome.
	connector.OAuthTokenJSON = sql.NullString{}
	_ = s.repo.Update(ctx, connector)
}

// --------------------------------------------------------------------------
// Tool registry bridge
// --------------------------------------------------------------------------

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

// --------------------------------------------------------------------------
// Tool discovery
// --------------------------------------------------------------------------

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

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

// listAllRefreshable returns connectors that should be probed in a health-check.
func (s *Service) listAllRefreshable(ctx context.Context) ([]domain.MCPConnector, error) {
	var result []domain.MCPConnector
	s.sessions.mu.RLock()
	ids := make([]string, 0, len(s.sessions.sessions))
	for id := range s.sessions.sessions {
		ids = append(ids, id)
	}
	s.sessions.mu.RUnlock()

	for _, id := range ids {
		c, err := s.repo.GetByID(ctx, id)
		if err != nil || c == nil {
			continue
		}
		if c.Refreshable() {
			result = append(result, *c)
		}
	}
	return result, nil
}

// buildHTTPClient creates an HTTP client with OAuth token injection if available.
func (s *Service) buildHTTPClient(connector *domain.MCPConnector) *http.Client {
	if !connector.OAuthTokenJSON.Valid || connector.OAuthTokenJSON.String == "" {
		return &http.Client{Timeout: 30 * time.Second}
	}

	var tokenResp OAuthTokenResponse
	if err := json.Unmarshal([]byte(connector.OAuthTokenJSON.String), &tokenResp); err != nil {
		return &http.Client{Timeout: 30 * time.Second}
	}

	if tokenResp.AccessToken == "" {
		return &http.Client{Timeout: 30 * time.Second}
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &BearerTokenTransport{
			Token: tokenResp.AccessToken,
		},
	}
}

// --------------------------------------------------------------------------
// Error classification
// --------------------------------------------------------------------------

// isAuthError checks if an error is an HTTP 401 Unauthorized.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "401") || strings.Contains(msg, "Unauthorized") || strings.Contains(msg, "unauthorized")
}

// isHTTPClientError checks for 4xx HTTP errors (excluding 401 which is handled separately).
func isHTTPClientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, code := range []string{"400", "403", "404", "405", "406", "409", "410", "422", "429"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}

// isTransientError determines whether an error is likely transient and worth
// retrying (DNS failures, connection resets, timeouts, 5xx).
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Network-level errors are transient.
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	msg := err.Error()
	transientPatterns := []string{
		"connection refused", "connection reset",
		"timeout", "i/o timeout", "deadline exceeded",
		"502", "503", "504",
		"eof", "broken pipe",
	}
	lower := strings.ToLower(msg)
	for _, p := range transientPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	return false
}

// validateExternalHost blocks requests to localhost, private RFC-1918 ranges,
// and cloud metadata endpoints.
func validateExternalHost(host string) error {
	host = strings.ToLower(host)
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("server URL cannot point to localhost")
	}
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "172.") {
		return fmt.Errorf("server URL cannot point to private network addresses")
	}
	if strings.HasPrefix(host, "169.254.") || host == "metadata.google.internal" {
		return fmt.Errorf("server URL cannot point to link-local or cloud metadata addresses")
	}
	return nil
}

// toNullString converts a string to sql.NullString.
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
