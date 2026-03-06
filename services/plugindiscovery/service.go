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
	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/toolregistry"
	"strings"
	"time"

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

	// maxConcurrentSessions caps simultaneous outbound MCP sessions per pod.
	// Prevents file-descriptor / memory exhaustion when many users trigger
	// tool calls concurrently. Analogous to agentstack's DB pool_size bounding.
	maxConcurrentSessions = 100

	// circuitFailureThreshold is how many consecutive failures trip the breaker.
	circuitFailureThreshold = 5

	// circuitRecoveryTimeout is how long the breaker stays open before allowing
	// a probe request (half-open state).
	circuitRecoveryTimeout = 60 * time.Second

	// maxMCPResponseBytes limits the size of any MCP response body read during
	// tool calls to prevent OOM from malicious or oversized servers.
	maxMCPResponseBytes = 16 << 20 // 16 MiB
)

// Service orchestrates MCP connector lifecycle: create, connect, discover tools, OAuth.
// Designed for horizontal scaling — no persistent sessions are held in memory.
// All MCP interactions use ephemeral sessions opened on-demand and closed after use.
//
// Scaling infrastructure (mirrors agentstack patterns):
//   - circuitRegistry: per-server-URL circuit breakers prevent thundering herd
//   - sessions semaphore: bounds concurrent outbound MCP sessions per pod
//   - presets: pre-configured connector templates for common MCP servers
type Service struct {
	log          *logger.Logger
	repo         domain.MCPConnectorRepository
	mcpClient    *mcp.Client
	oauth        *OAuthService
	toolRegistry *toolregistry.Registry
	circuits     *circuitRegistry
	sessions     *semaphore
	presets      []ConnectorPreset
}

// NewService creates a new MCP discovery service.
func NewService(log *logger.Logger, repo domain.MCPConnectorRepository, callbackURL string, toolRegistry *toolregistry.Registry) *Service {
	return &Service{
		log:  log,
		repo: repo,
		mcpClient: mcp.NewClient(
			&mcp.Implementation{Name: "schoolwithai", Version: "1.0.0"},
			&mcp.ClientOptions{},
		),
		oauth:        NewOAuthService(callbackURL),
		toolRegistry: toolRegistry,
		circuits:     newCircuitRegistry(circuitFailureThreshold, circuitRecoveryTimeout),
		sessions:     newSemaphore(maxConcurrentSessions),
		presets:      builtinPresets,
	}
}

// ListPresets returns the available connector presets.
func (s *Service) ListPresets() []ConnectorPreset {
	return s.presets
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
		return nil, fmt.Errorf("%w: maximum of %d per user", ErrConnectorLimitReached, maxConnectorsPerUser)
	}

	if title == "" {
		title = parsed.Host
	}

	// Auto-match against connector presets (agentstack's match_preset=true default).
	// If a preset matches, apply its metadata and credentials.
	var matchedPreset *ConnectorPreset
	if preset := MatchPreset(serverURL, s.presets); preset != nil {
		matchedPreset = preset
		if title == parsed.Host && preset.Name != "" {
			title = preset.Name
		}
	}

	connector := &domain.MCPConnector{
		UserID:    userID,
		Title:     title,
		ServerURL: serverURL,
		Status:    domain.ConnectorStatusCreated,
	}

	if err := s.repo.Create(ctx, connector); err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateConnector, serverURL)
		}
		return nil, fmt.Errorf("creating connector: %w", err)
	}

	s.log.Info(ctx, "mcp connector created", "id", connector.ID, "url", serverURL, "user_id", userID)

	// Apply preset credentials if the user didn't provide their own.
	if matchedPreset != nil && matchedPreset.ClientID != "" {
		if !connector.OAuthClientID.Valid || connector.OAuthClientID.String == "" {
			connector.OAuthClientID = toNullString(matchedPreset.ClientID)
			connector.OAuthClientSecret = toNullString(matchedPreset.ClientSecret)
			_ = s.repo.Update(ctx, connector)
		}
	}

	return connector, nil
}

// GetConnector retrieves a connector by ID.
func (s *Service) GetConnector(ctx context.Context, connectorID string) (*domain.MCPConnector, error) {
	return s.repo.GetByID(ctx, connectorID)
}

// ListConnectors returns all connectors for a user.
// Status is purely DB-driven — no in-memory session sync needed.
func (s *Service) ListConnectors(ctx context.Context, userID string) ([]domain.MCPConnector, error) {
	return s.repo.ListByUser(ctx, userID)
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
// An optional accessToken can be provided to skip OAuth — the token is injected
// directly as a Bearer token (agentstack pattern). If the injected token fails
// with a 401, the OAuth flow is initiated automatically.
func (s *Service) ConnectConnector(ctx context.Context, connectorID, accessToken string) (*domain.MCPConnector, string, error) {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return nil, "", fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, "", ErrConnectorNotFound
	}
	connector.Status = domain.ConnectorStatusConnecting
	connector.ErrorMsg = ""

	// If an access_token was provided, inject it directly so buildHTTPClient
	// can use it as a Bearer token (agentstack pattern — skip OAuth).
	if accessToken != "" {
		tokenResp := OAuthTokenResponse{
			AccessToken: accessToken,
			TokenType:   "bearer",
		}
		tokenJSON, _ := json.Marshal(tokenResp)
		connector.OAuthTokenJSON = toNullString(string(tokenJSON))
	}

	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector status: %w", err)
	}

	// Build HTTP client (with OAuth token if available).
	httpClient := s.buildHTTPClient(connector)

	// Check circuit breaker — if the remote server has been failing, fast-fail
	// instead of adding to the thundering herd.
	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		connector.ErrorMsg = "server circuit breaker open — recent failures detected"
		_ = connector.Transition(domain.ConnectorStatusError, "", false)
		_ = s.repo.Update(ctx, connector)
		return connector, "", ErrCircuitOpen
	}

	// Acquire concurrency slot to bound outbound sessions per pod.
	if err := s.sessions.Acquire(ctx); err != nil {
		return nil, "", err
	}
	defer s.sessions.Release()

	// Open ephemeral session with retries — validates reachability and auth.
	// The session is used for metadata capture and tool discovery, then closed.
	// No persistent connection is kept (stateless proxy pattern).
	session, err := s.probeConnector(ctx, connector, httpClient)
	if err != nil {
		cb.RecordFailure()
		return s.handleConnectionError(ctx, connector, err)
	}
	defer session.Close()
	cb.RecordSuccess()

	// Capture server metadata from the initialization result.
	initResult := session.InitializeResult()
	if initResult != nil {
		connector.ServerName = toNullString(initResult.ServerInfo.Name)
		connector.ServerVersion = toNullString(initResult.ServerInfo.Version)
		capJSON, _ := json.Marshal(initResult.Capabilities)
		connector.Capabilities = toNullString(string(capJSON))
	}

	// Discover tools and cache them in the DB for permission lookups.
	if err := s.discoverTools(ctx, connector, session); err != nil {
		s.log.Warn(ctx, "tool discovery failed", "connector_id", connectorID, "error", err)
		connector.ErrorMsg = "connected but tool discovery failed: " + err.Error()
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

// DisconnectConnector revokes tokens and updates status to disconnected.
// Returns the updated connector (agentstack pattern — caller can use it for response).
func (s *Service) DisconnectConnector(ctx context.Context, connectorID string) (*domain.MCPConnector, error) {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, ErrConnectorNotFound
	}

	// Validate that the connector is in a state that can be disconnected.
	// Agentstack validates status ∈ {connected, connecting, auth_required, disconnected}.
	switch connector.Status {
	case domain.ConnectorStatusConnected, domain.ConnectorStatusConnecting,
		domain.ConnectorStatusAuthRequired, domain.ConnectorStatusDisconnected:
		// OK — proceed.
	case domain.ConnectorStatusCreated:
		return nil, fmt.Errorf("%w: connector has never been connected", ErrInvalidConnectorState)
	default:
		// error state — allow disconnect to reset.
	}

	// Remove from plugin system.
	s.unregisterConnectorFromToolRegistry(ctx, connectorID)

	// Revoke OAuth token if present.
	s.revokeToken(ctx, connector)

	// Clear pending OAuth flow state so stale state can't be replayed
	// (agentstack clears auth_flow on disconnect).
	connector.OAuthState = ""
	connector.OAuthCodeVerifier = ""

	_ = connector.Transition(domain.ConnectorStatusDisconnected, "client request", true)
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, fmt.Errorf("updating connector status: %w", err)
	}

	s.log.Info(ctx, "mcp connector disconnected", "id", connectorID)
	return connector, nil
}

// DeleteConnector disconnects and removes a connector.
func (s *Service) DeleteConnector(ctx context.Context, connectorID, userID string) error {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return ErrConnectorNotFound
	}
	if connector.UserID != userID {
		return ErrConnectorNotFound // don't leak existence to other users
	}

	s.unregisterConnectorFromToolRegistry(ctx, connectorID)
	s.revokeToken(ctx, connector)
	s.circuits.Reset(connector.ServerURL)

	if err := s.repo.Delete(ctx, connectorID); err != nil {
		return fmt.Errorf("deleting connector: %w", err)
	}

	s.log.Info(ctx, "mcp connector deleted", "id", connectorID, "user_id", userID)
	return nil
}

// RefreshTools re-discovers tools from a connected MCP server via an
// ephemeral session. The discovered tools are cached in the DB.
func (s *Service) RefreshTools(ctx context.Context, connectorID string) ([]domain.MCPDiscoveredTool, error) {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, ErrConnectorNotFound
	}
	if connector.Status != domain.ConnectorStatusConnected {
		return nil, ErrConnectorNotConnected
	}

	// Check circuit breaker before opening a session.
	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		return nil, ErrCircuitOpen
	}

	// Bound concurrent sessions.
	if err := s.sessions.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.sessions.Release()

	httpClient := s.buildHTTPClient(connector)
	session, err := s.openEphemeralSession(ctx, connector.ServerURL, httpClient)
	if err != nil {
		cb.RecordFailure()
		return nil, fmt.Errorf("opening session for tool refresh: %w", err)
	}
	defer session.Close()
	cb.RecordSuccess()

	if err := s.discoverTools(ctx, connector, session); err != nil {
		return nil, fmt.Errorf("refreshing tools: %w", err)
	}

	return s.repo.ListTools(ctx, connectorID)
}

// --------------------------------------------------------------------------
// OAuth flow
// --------------------------------------------------------------------------

// HandleOAuthCallback processes the OAuth redirect callback.
func (s *Service) HandleOAuthCallback(ctx context.Context, stateParam, code string) (*domain.MCPConnector, error) {
	// Look up the connector by persisted OAuth state (survives restarts).
	connector, err := s.repo.GetByOAuthState(ctx, stateParam)
	if err != nil {
		return nil, fmt.Errorf("looking up oauth state: %w", err)
	}
	if connector == nil {
		return nil, ErrOAuthStateInvalid
	}

	// Agentstack validates the connector must be in auth_required state.
	if connector.Status != domain.ConnectorStatusAuthRequired {
		return nil, fmt.Errorf("%w: expected auth_required, got %s", ErrInvalidConnectorState, connector.Status)
	}

	// Extract the code verifier before clearing it.
	codeVerifier := connector.OAuthCodeVerifier

	// Clear the state fields so they can't be replayed.
	connector.OAuthState = ""
	connector.OAuthCodeVerifier = ""

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
	tokenResp, err := s.oauth.ExchangeCode(ctx, &authMeta, clientID, clientSecret, code, codeVerifier)
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

	// Reconnect with tokens (no external access_token — we just obtained one via OAuth).
	_, _, err = s.ConnectConnector(ctx, connector.ID, "")
	if err != nil {
		return nil, fmt.Errorf("reconnecting after OAuth: %w", err)
	}

	return s.repo.GetByID(ctx, connector.ID)
}

// --------------------------------------------------------------------------
// Tool execution
// --------------------------------------------------------------------------

// CallTool executes a tool on a remote MCP server via an ephemeral session.
// Each call opens a fresh session, executes, and closes — no persistent
// connection is held (stateless proxy pattern for horizontal scaling).
func (s *Service) CallTool(ctx context.Context, connectorID, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	connector, err := s.repo.GetByID(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, ErrConnectorNotFound
	}
	if connector.Status != domain.ConnectorStatusConnected {
		return nil, ErrConnectorNotConnected
	}

	// Fast-fail if the circuit breaker is open for this server.
	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		return nil, ErrCircuitOpen
	}

	// Bound concurrent outbound sessions.
	if err := s.sessions.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.sessions.Release()

	httpClient := s.buildHTTPClient(connector)
	session, err := s.openEphemeralSession(ctx, connector.ServerURL, httpClient)
	if err != nil {
		cb.RecordFailure()
		return nil, fmt.Errorf("opening ephemeral session: %w", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		cb.RecordFailure()
		return nil, fmt.Errorf("calling tool %s: %w", toolName, err)
	}
	cb.RecordSuccess()
	return result, nil
}

// ResolveEnabledTools returns MCP executable tools for a user, filtered by the provided
// permission checker callbacks. Tool metadata comes from the DB cache (populated on
// connect/refresh). Tool execution uses ephemeral sessions — no persistent connections.
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

		discoveredTools, err := s.GetConnectorTools(ctx, conn.ID)
		if err != nil {
			continue
		}

		for _, dt := range discoveredTools {
			toolID := "mcp-" + conn.ID + "-" + dt.Name
			if !isToolEnabled(toolSetID, toolID) {
				continue
			}
			tools = append(tools, NewMCPRemoteTool(s, conn.ID, dt.Name, dt.Description, dt.InputSchema))
		}
	}
	return tools
}

// --------------------------------------------------------------------------
// Probe & connection error handling
// --------------------------------------------------------------------------

// probeConnector performs a lightweight MCP handshake to verify that the server
// is reachable and the credentials are valid. It retries on transient errors.
// On success, returns the live session — caller is responsible for closing it.
func (s *Service) probeConnector(ctx context.Context, connector *domain.MCPConnector, httpClient *http.Client) (*mcp.ClientSession, error) {
	var lastErr error

	for attempt := range probeMaxRetries {
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		transport := &mcp.StreamableClientTransport{
			Endpoint: connector.ServerURL,
		}
		if httpClient != nil {
			transport.HTTPClient = httpClient
		}

		session, err := s.mcpClient.Connect(probeCtx, transport, nil)
		if err == nil {
			cancel()
			return session, nil
		}
		cancel()
		lastErr = err

		// Only retry on transient network errors.
		if !isTransientError(err) {
			return nil, lastErr
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

	return nil, lastErr
}

// openEphemeralSession opens a short-lived MCP session to the given server.
// The caller MUST close the session when done. This is the core primitive for
// the stateless proxy pattern — sessions are opened on-demand and closed after use.
func (s *Service) openEphemeralSession(ctx context.Context, serverURL string, httpClient *http.Client) (*mcp.ClientSession, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint: serverURL,
	}
	if httpClient != nil {
		transport.HTTPClient = httpClient
	} else {
		transport.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return s.mcpClient.Connect(ctx, transport, nil)
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

	// Persist the OAuth state to the connector row so it survives server
	// restarts and works across multi-instance deployments (agentstack pattern).
	connector.OAuthState = state
	connector.OAuthCodeVerifier = codeVerifier

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

// buildHTTPClient creates an HTTP client with OAuth token injection if available.
// When a refresh token and auth server metadata are present, the client uses an
// auto-refreshing transport (like agentstack's auto_refresh_client) that
// transparently refreshes expired tokens and persists the result.
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

	// If we have a refresh token AND cached auth server metadata, use the
	// auto-refreshing transport so expired tokens are refreshed transparently.
	if tokenResp.RefreshToken != "" && connector.OAuthServerMeta.Valid {
		var authMeta OAuthMetadata
		if err := json.Unmarshal([]byte(connector.OAuthServerMeta.String), &authMeta); err == nil {
			clientID := ""
			if connector.OAuthClientID.Valid {
				clientID = connector.OAuthClientID.String
			}
			clientSecret := ""
			if connector.OAuthClientSecret.Valid {
				clientSecret = connector.OAuthClientSecret.String
			}

			connectorID := connector.ID
			return &http.Client{
				Timeout: 30 * time.Second,
				Transport: NewAutoRefreshTransport(
					s.oauth, &authMeta, clientID, clientSecret, tokenResp,
					func(newToken *OAuthTokenResponse) error {
						// Persist refreshed token — best-effort.
						tokenJSON, _ := json.Marshal(newToken)
						connector.OAuthTokenJSON = toNullString(string(tokenJSON))
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if err := s.repo.Update(ctx, connector); err != nil {
							s.log.Warn(ctx, "failed to persist refreshed token",
								"connector_id", connectorID, "error", err)
							return err
						}
						return nil
					},
				),
			}
		}
	}

	// Fallback: static bearer token (no refresh capability).
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
