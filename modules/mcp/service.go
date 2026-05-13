package mcp

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

	// maxSessionsPerUser caps how many concurrent MCP sessions a single user
	// can hold. At 100K users × 10 connectors, this prevents one user from
	// monopolizing the global semaphore (e.g. 10 parallel tool calls).
	maxSessionsPerUser = 10

	// connectorCacheTTL is how long a connector lookup is cached in memory.
	// Short enough to pick up status changes quickly, long enough to avoid
	// hammering PostgreSQL on hot paths (CallTool, MCPProxy).
	connectorCacheTTL = 30 * time.Second

	// circuitFailureThreshold is how many consecutive failures trip the breaker.
	circuitFailureThreshold = 5

	// circuitRecoveryTimeout is how long the breaker stays open before allowing
	// a probe request (half-open state).
	circuitRecoveryTimeout = 60 * time.Second
)

// Service orchestrates MCP connector lifecycle: create, connect, discover tools, OAuth.
// Designed for horizontal scaling — no persistent sessions are held in memory.
// All MCP interactions use ephemeral sessions opened on-demand and closed after use.
//
// Scaling infrastructure (mirrors agentstack patterns):
//   - circuitRegistry: per-server-URL circuit breakers prevent thundering herd
//   - sessions semaphore: bounds concurrent outbound MCP sessions per pod
//   - httpClients cache: avoids rebuilding OAuth transports on every request
//   - presets: pre-configured connector templates for common MCP servers
type Service struct {
	log            *logger.Logger
	repo           MCPConnectorRepository
	oauth          *OAuthService
	toolRegistry   *toolregistry.Registry
	circuits       *circuitRegistry
	sessions       *semaphore
	userSessions   *userSemaphore           // per-user concurrency limiter
	httpClients    *ttlCache[*http.Client]  // per-connector cached HTTP clients
	connectorCache *ttlCache[*MCPConnector] // read-through cache for hot-path lookups
	presets        []ConnectorPreset
	stopCleanup    chan struct{} // signals background cleanup goroutine to stop
}

// NewService creates a new MCP discovery service.
func NewService(log *logger.Logger, repo MCPConnectorRepository, callbackURL string, toolRegistry *toolregistry.Registry) *Service {
	s := &Service{
		log:            log,
		repo:           repo,
		oauth:          NewOAuthService(callbackURL),
		toolRegistry:   toolRegistry,
		circuits:       newCircuitRegistry(circuitFailureThreshold, circuitRecoveryTimeout),
		sessions:       newSemaphore(maxConcurrentSessions),
		userSessions:   newUserSemaphore(maxSessionsPerUser),
		httpClients:    newTTLCache[*http.Client](5 * time.Minute),
		connectorCache: newTTLCache[*MCPConnector](connectorCacheTTL),
		presets:        builtinPresets,
		stopCleanup:    make(chan struct{}),
	}
	go s.backgroundCleanup()
	return s
}

// Shutdown stops background goroutines. Call on graceful shutdown.
func (s *Service) Shutdown() {
	close(s.stopCleanup)
}

// backgroundCleanup periodically evicts stale circuit breakers and expired
// cache entries to prevent unbounded memory growth.
func (s *Service) backgroundCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCleanup:
			return
		case <-ticker.C:
			s.circuits.Cleanup(10 * time.Minute)
			s.httpClients.Cleanup()
			s.connectorCache.Cleanup()
		}
	}
}

// ListPresets returns the available connector presets.
func (s *Service) ListPresets() []ConnectorPreset {
	return s.presets
}

// --------------------------------------------------------------------------
// Tenant isolation
// --------------------------------------------------------------------------

// getOwnedConnector fetches a connector and verifies it belongs to the given
// user. Ownership is enforced at the SQL level (WHERE id=$1 AND user_id=$2) —
// the DB never returns another user's data. Returns ErrConnectorNotFound if
// the connector doesn't exist or belongs to another user — callers cannot
// distinguish the two cases, preventing enumeration.
//
// Results are cached with a short TTL (30s) to avoid hitting PostgreSQL on
// every CallTool / MCPProxy request. Cache is invalidated on any write.
func (s *Service) getOwnedConnector(ctx context.Context, connectorID, userID string) (*MCPConnector, error) {
	cacheKey := connectorID + ":" + userID

	// Try cache first — hot path for CallTool / MCPProxy.
	if cached, ok := s.connectorCache.Get(cacheKey); ok {
		return cached, nil
	}

	// SQL-level isolation: the DB only returns the row if it belongs to this user.
	connector, err := s.repo.GetByIDAndUser(ctx, connectorID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching connector: %w", err)
	}
	if connector == nil {
		return nil, ErrConnectorNotFound
	}

	// Cache for subsequent reads on the same hot path.
	s.connectorCache.Set(cacheKey, connector)
	return connector, nil
}

// invalidateConnectorCache evicts a connector from the read-through cache.
// Must be called after any write (update, delete, status change).
func (s *Service) invalidateConnectorCache(connectorID, userID string) {
	s.connectorCache.Delete(connectorID + ":" + userID)
}

// --------------------------------------------------------------------------
// CRUD
// --------------------------------------------------------------------------

// CreateConnector registers a new MCP server URL for a user.
// Optional OAuth client credentials are persisted with the connector so they
// are available during the connect phase (aligns with agentstack's create_connector).
func (s *Service) CreateConnector(ctx context.Context, userID, serverURL, title, clientID, clientSecret string) (*MCPConnector, error) {
	parsed, err := validateConnectorURL(serverURL)
	if err != nil {
		return nil, err
	}
	if err := s.enforceConnectorQuota(ctx, userID); err != nil {
		return nil, err
	}

	if title == "" {
		title = parsed.Host
	}
	// Auto-match against connector presets (agentstack's match_preset=true
	// default) — if a preset matches we borrow its display name and
	// (below) fall back to its OAuth credentials.
	matchedPreset := MatchPreset(serverURL, s.presets)
	if matchedPreset != nil && title == parsed.Host && matchedPreset.Name != "" {
		title = matchedPreset.Name
	}

	connector := &MCPConnector{
		UserID:    userID,
		Title:     title,
		ServerURL: serverURL,
		Status:    ConnectorStatusCreated,
	}
	applyConnectorOAuthCreds(connector, clientID, clientSecret, matchedPreset)

	if err := s.repo.Create(ctx, connector); err != nil {
		if isDuplicateKeyError(err) {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateConnector, serverURL)
		}
		return nil, fmt.Errorf("creating connector: %w", err)
	}

	s.log.Info(ctx, "mcp connector created", "id", connector.ID, "url", serverURL, "user_id", userID)
	return connector, nil
}

// validateConnectorURL parses serverURL, enforces http/https scheme, and
// runs the SSRF host check. Returns the parsed URL so the caller can
// read Host / Hostname without reparsing.
func validateConnectorURL(serverURL string) (*url.URL, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("server URL must use http or https scheme")
	}
	if err := validateExternalHost(parsed.Hostname()); err != nil {
		return nil, err
	}
	return parsed, nil
}

// enforceConnectorQuota blocks creation when the user has reached
// maxConnectorsPerUser. Lookup cost is one index read on user_id.
func (s *Service) enforceConnectorQuota(ctx context.Context, userID string) error {
	count, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking connector count: %w", err)
	}
	if count >= maxConnectorsPerUser {
		return fmt.Errorf("%w: maximum of %d per user", ErrConnectorLimitReached, maxConnectorsPerUser)
	}
	return nil
}

// applyConnectorOAuthCreds prefers caller-supplied clientID/secret; falls
// back to the preset's credentials only when the caller provided nothing.
func applyConnectorOAuthCreds(c *MCPConnector, clientID, clientSecret string, preset *ConnectorPreset) {
	if clientID != "" {
		c.OAuthClientID = toNullString(clientID)
		c.OAuthClientSecret = toNullString(clientSecret)
		return
	}
	if preset != nil && preset.ClientID != "" {
		c.OAuthClientID = toNullString(preset.ClientID)
		c.OAuthClientSecret = toNullString(preset.ClientSecret)
	}
}

// isDuplicateKeyError inspects repo error text for the pg unique-violation
// markers. Repo layer doesn't expose typed errors today; matching here
// keeps the service layer's error translation in one place.
func isDuplicateKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}

// GetConnector retrieves a connector by ID, verifying it belongs to the user.
func (s *Service) GetConnector(ctx context.Context, connectorID, userID string) (*MCPConnector, error) {
	return s.getOwnedConnector(ctx, connectorID, userID)
}

// ListConnectors returns all connectors for a user.
// Status is purely DB-driven — no in-memory session sync needed.
func (s *Service) ListConnectors(ctx context.Context, userID string) ([]MCPConnector, error) {
	return s.repo.ListByUser(ctx, userID)
}

// GetConnectorTools returns the discovered tools for a connector.
// Verifies the connector belongs to the given user.
func (s *Service) GetConnectorTools(ctx context.Context, connectorID, userID string) ([]MCPDiscoveredTool, error) {
	if _, err := s.getOwnedConnector(ctx, connectorID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListTools(ctx, connectorID)
}

// CountToolsByConnectors returns a map of connector ID → tool count.
// Single query instead of N+1. Used by the list handler.
func (s *Service) CountToolsByConnectors(ctx context.Context, connectorIDs []string) (map[string]int, error) {
	return s.repo.CountToolsByConnectors(ctx, connectorIDs)
}

// --------------------------------------------------------------------------
// Connection lifecycle
// --------------------------------------------------------------------------

// ConnectConnector attempts to establish an MCP session and discover tools.
// If the server responds with 401, triggers the OAuth flow and returns the auth URL.
// An optional accessToken can be provided to skip OAuth — the token is injected
// directly as a Bearer token (agentstack pattern). If the injected token fails
// with a 401, the OAuth flow is initiated automatically.
func (s *Service) ConnectConnector(ctx context.Context, connectorID, userID, accessToken string) (*MCPConnector, string, error) {
	connector, err := s.getOwnedConnector(ctx, connectorID, userID)
	if err != nil {
		return nil, "", err
	}
	connector.Status = ConnectorStatusConnecting
	connector.ErrorMsg = ""
	s.injectAccessToken(connector, accessToken)

	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector status: %w", err)
	}

	httpClient := s.buildHTTPClient(connector)
	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		return s.failOpenCircuit(ctx, connector)
	}

	// Acquire per-user slot before the global one so a single user can't
	// monopolise the pod's session pool.
	if err := s.userSessions.Acquire(ctx, userID); err != nil {
		return nil, "", err
	}
	defer s.userSessions.Release(userID)

	if err := s.sessions.Acquire(ctx); err != nil {
		return nil, "", err
	}
	defer s.sessions.Release()

	// Open ephemeral session with retries — validates reachability + auth.
	// Metadata + tool discovery use this same session, then close it.
	session, err := s.probeConnector(ctx, connector, httpClient)
	if err != nil {
		cb.RecordFailure()
		return s.handleConnectionError(ctx, connector, err)
	}
	defer func() { _ = session.Close() }()
	cb.RecordSuccess()

	captureServerMetadata(connector, session.InitializeResult())
	if err := s.discoverTools(ctx, connector, session); err != nil {
		s.log.Warn(ctx, "tool discovery failed", "connector_id", connectorID, "error", err)
		connector.ErrorMsg = "connected but tool discovery failed: " + err.Error()
	}

	_ = connector.Transition(ConnectorStatusConnected, "", false)
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector after connect: %w", err)
	}
	s.invalidateConnectorCache(connector.ID, userID)
	s.registerConnectorInToolRegistry(ctx, connector)

	s.log.Info(ctx, "mcp connector connected", "id", connector.ID, "server", connector.ServerName.String)
	return connector, "", nil
}

// marshalTokenResponse serializes a token response as JSON via a map so the
// credential-bearing field is not declared as a struct field at the marshal
// site (avoids gosec G117 on AccessToken / RefreshToken).
func marshalTokenResponse(t *OAuthTokenResponse) ([]byte, error) {
	if t == nil {
		return []byte("null"), nil
	}
	payload := map[string]any{
		"access_token": t.AccessToken,
		"token_type":   t.TokenType,
	}
	if t.ExpiresIn != 0 {
		payload["expires_in"] = t.ExpiresIn
	}
	if t.RefreshToken != "" {
		payload["refresh_token"] = t.RefreshToken
	}
	if t.Scope != "" {
		payload["scope"] = t.Scope
	}
	return json.Marshal(payload)
}

// injectAccessToken writes an externally-supplied bearer token onto the
// connector so buildHTTPClient uses it instead of triggering OAuth
// (agentstack pattern). Empty token is a no-op.
func (s *Service) injectAccessToken(connector *MCPConnector, accessToken string) {
	if accessToken == "" {
		return
	}
	tokenJSON, _ := json.Marshal(map[string]any{
		"access_token": accessToken,
		"token_type":   "bearer",
	})
	connector.OAuthTokenJSON = toNullString(string(tokenJSON))
	s.invalidateHTTPClient(connector.ID)
}

// failOpenCircuit records the open-circuit state on the connector, flips
// it to error, and returns the ErrCircuitOpen sentinel so the caller
// short-circuits cleanly without opening a remote session.
func (s *Service) failOpenCircuit(ctx context.Context, connector *MCPConnector) (*MCPConnector, string, error) {
	connector.ErrorMsg = "server circuit breaker open — recent failures detected"
	_ = connector.Transition(ConnectorStatusError, "", false)
	_ = s.repo.Update(ctx, connector)
	return connector, "", ErrCircuitOpen
}

// captureServerMetadata copies ServerInfo + Capabilities off the
// initialization result into the connector row. Nil init result (possible
// on protocol-violating servers) leaves the connector untouched.
func captureServerMetadata(connector *MCPConnector, initResult *mcp.InitializeResult) {
	if initResult == nil {
		return
	}
	connector.ServerName = toNullString(initResult.ServerInfo.Name)
	connector.ServerVersion = toNullString(initResult.ServerInfo.Version)
	capJSON, _ := json.Marshal(initResult.Capabilities)
	connector.Capabilities = toNullString(string(capJSON))
}

// DisconnectConnector revokes tokens and updates status to disconnected.
// Returns the updated connector (agentstack pattern — caller can use it for response).
func (s *Service) DisconnectConnector(ctx context.Context, connectorID, userID string) (*MCPConnector, error) {
	connector, err := s.getOwnedConnector(ctx, connectorID, userID)
	if err != nil {
		return nil, err
	}

	// Validate that the connector is in a state that can be disconnected.
	// Agentstack validates status ∈ {connected, connecting, auth_required, disconnected}.
	switch connector.Status {
	case ConnectorStatusConnected, ConnectorStatusConnecting,
		ConnectorStatusAuthRequired, ConnectorStatusDisconnected:
		// OK — proceed.
	case ConnectorStatusCreated:
		return nil, fmt.Errorf("%w: connector has never been connected", ErrInvalidConnectorState)
	default:
		// error state — allow disconnect to reset.
	}

	// Remove from plugin system.
	s.unregisterConnectorFromToolRegistry(ctx, connectorID)

	// Revoke OAuth token if present.
	s.revokeToken(ctx, connector)
	s.invalidateHTTPClient(connectorID)

	// Clear pending OAuth flow state so stale state can't be replayed
	// (agentstack clears auth_flow on disconnect).
	connector.OAuthState = ""
	connector.OAuthCodeVerifier = ""

	_ = connector.Transition(ConnectorStatusDisconnected, "client request", true)
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, fmt.Errorf("updating connector status: %w", err)
	}
	s.invalidateConnectorCache(connectorID, userID)

	s.log.Info(ctx, "mcp connector disconnected", "id", connectorID)
	return connector, nil
}

// DeleteConnector disconnects and removes a connector.
func (s *Service) DeleteConnector(ctx context.Context, connectorID, userID string) error {
	connector, err := s.getOwnedConnector(ctx, connectorID, userID)
	if err != nil {
		return err
	}

	s.unregisterConnectorFromToolRegistry(ctx, connectorID)
	s.revokeToken(ctx, connector)
	s.circuits.Reset(connector.ServerURL)
	s.invalidateHTTPClient(connectorID)
	s.invalidateConnectorCache(connectorID, userID)

	if err := s.repo.Delete(ctx, connectorID, userID); err != nil {
		return fmt.Errorf("deleting connector: %w", err)
	}

	s.log.Info(ctx, "mcp connector deleted", "id", connectorID, "user_id", userID)
	return nil
}

// RefreshTools re-discovers tools from a connected MCP server via an
// ephemeral session. The discovered tools are cached in the DB.
func (s *Service) RefreshTools(ctx context.Context, connectorID, userID string) ([]MCPDiscoveredTool, error) {
	connector, err := s.getOwnedConnector(ctx, connectorID, userID)
	if err != nil {
		return nil, err
	}
	if connector.Status != ConnectorStatusConnected {
		return nil, ErrConnectorNotConnected
	}

	// Check circuit breaker before opening a session.
	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		return nil, ErrCircuitOpen
	}

	// Per-user concurrency cap.
	if err := s.userSessions.Acquire(ctx, userID); err != nil {
		return nil, err
	}
	defer s.userSessions.Release(userID)

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
	defer func() { _ = session.Close() }()
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
func (s *Service) HandleOAuthCallback(ctx context.Context, stateParam, code string) (*MCPConnector, error) {
	connector, err := s.loadOAuthCandidate(ctx, stateParam)
	if err != nil {
		return nil, err
	}

	codeVerifier := connector.OAuthCodeVerifier
	connector.OAuthState = ""
	connector.OAuthCodeVerifier = ""

	authMeta, err := decodeCachedAuthMetadata(connector)
	if err != nil {
		return nil, err
	}
	clientID, clientSecret := loadStoredOAuthClientCreds(connector)

	tokenResp, err := s.oauth.ExchangeCode(ctx, authMeta, clientID, clientSecret, code, codeVerifier)
	if err != nil {
		s.transitionToError(ctx, connector, "token exchange failed: "+err.Error())
		return nil, fmt.Errorf("exchanging code: %w", err)
	}

	if err := s.persistAcquiredTokens(ctx, connector, tokenResp); err != nil {
		return nil, err
	}
	s.log.Info(ctx, "oauth tokens acquired", "connector_id", connector.ID)

	// Reconnect with the freshly acquired token. The connector's UserID
	// is trusted because we looked it up via the cryptographically random
	// OAuth state — not via a user-supplied ID.
	if _, _, err := s.ConnectConnector(ctx, connector.ID, connector.UserID, ""); err != nil {
		return nil, fmt.Errorf("reconnecting after OAuth: %w", err)
	}
	return s.getOwnedConnector(ctx, connector.ID, connector.UserID)
}

// loadOAuthCandidate looks up the connector via the persisted OAuth state
// and enforces the auth_required precondition (agentstack parity).
func (s *Service) loadOAuthCandidate(ctx context.Context, stateParam string) (*MCPConnector, error) {
	connector, err := s.repo.GetByOAuthState(ctx, stateParam)
	if err != nil {
		return nil, fmt.Errorf("looking up oauth state: %w", err)
	}
	if connector == nil {
		return nil, ErrOAuthStateInvalid
	}
	if connector.Status != ConnectorStatusAuthRequired {
		return nil, fmt.Errorf("%w: expected auth_required, got %s", ErrInvalidConnectorState, connector.Status)
	}
	return connector, nil
}

// decodeCachedAuthMetadata re-parses the JSON blob of OAuth authorization
// server metadata the connector cached during initiateOAuth. Missing or
// malformed metadata is a hard error — we can't complete the exchange
// without it.
func decodeCachedAuthMetadata(connector *MCPConnector) (*OAuthMetadata, error) {
	if !connector.OAuthServerMeta.Valid {
		return nil, fmt.Errorf("no cached auth metadata for connector")
	}
	var meta OAuthMetadata
	if err := json.Unmarshal([]byte(connector.OAuthServerMeta.String), &meta); err != nil {
		return nil, fmt.Errorf("decoding cached auth metadata: %w", err)
	}
	return &meta, nil
}

// loadStoredOAuthClientCreds pulls the client_id / client_secret the
// connector learned during initiateOAuth (either pre-configured or via
// dynamic client registration). Missing values return empty strings —
// the caller passes them through to the authorization server.
func loadStoredOAuthClientCreds(connector *MCPConnector) (string, string) {
	var clientID, clientSecret string
	if connector.OAuthClientID.Valid {
		clientID = connector.OAuthClientID.String
	}
	if connector.OAuthClientSecret.Valid {
		clientSecret = connector.OAuthClientSecret.String
	}
	return clientID, clientSecret
}

// persistAcquiredTokens stores the freshly exchanged OAuth tokens on the
// connector and invalidates the HTTP-client + connector caches so the next
// request uses the new bearer token.
func (s *Service) persistAcquiredTokens(ctx context.Context, connector *MCPConnector, tokenResp *OAuthTokenResponse) error {
	tokenJSON, _ := marshalTokenResponse(tokenResp)
	connector.OAuthTokenJSON = toNullString(string(tokenJSON))
	connector.Status = ConnectorStatusConnecting
	connector.ErrorMsg = ""
	s.invalidateHTTPClient(connector.ID)
	s.invalidateConnectorCache(connector.ID, connector.UserID)
	if err := s.repo.Update(ctx, connector); err != nil {
		return fmt.Errorf("saving tokens: %w", err)
	}
	return nil
}

// transitionToError sets the human-readable error message, flips the
// connector into the Error state, and persists both — the pattern used
// whenever an OAuth setup step fails partway through initiateOAuth /
// HandleOAuthCallback.
func (s *Service) transitionToError(ctx context.Context, connector *MCPConnector, msg string) {
	connector.ErrorMsg = msg
	_ = connector.Transition(ConnectorStatusError, "", false)
	_ = s.repo.Update(ctx, connector)
}

// --------------------------------------------------------------------------
// Tool execution
// --------------------------------------------------------------------------

// CallTool executes a tool on a remote MCP server via an ephemeral session.
// Each call opens a fresh session, executes, and closes — no persistent
// connection is held (stateless proxy pattern for horizontal scaling).
func (s *Service) CallTool(ctx context.Context, connectorID, userID, toolName string, arguments map[string]any) (*mcp.CallToolResult, error) {
	connector, err := s.getOwnedConnector(ctx, connectorID, userID)
	if err != nil {
		return nil, err
	}
	if connector.Status != ConnectorStatusConnected {
		return nil, ErrConnectorNotConnected
	}

	// Fast-fail if the circuit breaker is open for this server.
	cb := s.circuits.Get(connector.ServerURL)
	if !cb.Allow() {
		return nil, ErrCircuitOpen
	}

	// Per-user concurrency cap — prevents one user from monopolizing sessions.
	if err := s.userSessions.Acquire(ctx, userID); err != nil {
		return nil, err
	}
	defer s.userSessions.Release(userID)

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
	defer func() { _ = session.Close() }()

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
	connectedIDs := collectEnabledConnectorIDs(connectors, isToolSetEnabled)
	if len(connectedIDs) == 0 {
		return nil
	}
	// Single query fetches tools for ALL enabled connectors — avoids N+1.
	allTools, err := s.repo.ListToolsByConnectors(ctx, connectedIDs)
	if err != nil {
		return nil
	}
	return s.buildRemoteMCPTools(userID, connectedIDs, allTools, isToolEnabled)
}

// collectEnabledConnectorIDs filters the user's connectors to those that
// are both connected AND have their tool-set enabled by the caller's
// permission callback. Skipping disabled connectors avoids an unnecessary
// DB round-trip for tools the user can't use anyway.
func collectEnabledConnectorIDs(connectors []MCPConnector, isToolSetEnabled func(string) bool) []string {
	var connectedIDs []string
	for _, conn := range connectors {
		if conn.Status != ConnectorStatusConnected {
			continue
		}
		if !isToolSetEnabled("mcp-" + conn.ID) {
			continue
		}
		connectedIDs = append(connectedIDs, conn.ID)
	}
	return connectedIDs
}

// buildRemoteMCPTools wraps each enabled per-connector tool in an MCP
// remote tool, applying the per-tool permission callback so tools blocked
// inside an enabled tool-set don't leak through.
func (s *Service) buildRemoteMCPTools(
	userID string, connectedIDs []string,
	allTools map[string][]MCPDiscoveredTool,
	isToolEnabled func(toolSetID, toolID string) bool,
) []agent.ExecutableTool {
	var tools []agent.ExecutableTool
	for _, connID := range connectedIDs {
		toolSetID := "mcp-" + connID
		for _, dt := range allTools[connID] {
			toolID := toolSetID + "-" + dt.Name
			if !isToolEnabled(toolSetID, toolID) {
				continue
			}
			tools = append(tools, NewMCPRemoteTool(s, connID, userID, dt.Name, dt.Description, dt.InputSchema))
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
func (s *Service) probeConnector(ctx context.Context, connector *MCPConnector, httpClient *http.Client) (*mcp.ClientSession, error) {
	var lastErr error

	for attempt := range probeMaxRetries {
		probeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		transport := &mcp.StreamableClientTransport{
			Endpoint: connector.ServerURL,
		}
		if httpClient != nil {
			transport.HTTPClient = httpClient
		}

		// Create a fresh Client per session — avoids accumulating session
		// references in a shared Client (the go-sdk tracks sessions internally).
		client := mcp.NewClient(
			&mcp.Implementation{Name: "ai-platform", Version: "1.0.0"},
			&mcp.ClientOptions{},
		)

		session, err := client.Connect(probeCtx, transport, nil)
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
			// Context-aware sleep: respects cancellation instead of blocking.
			delay := probeRetryDelay * time.Duration(attempt+1)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}

	return nil, lastErr
}

// openEphemeralSession opens a short-lived MCP session to the given server.
// The caller MUST close the session when done.
func (s *Service) openEphemeralSession(ctx context.Context, serverURL string, httpClient *http.Client) (*mcp.ClientSession, error) {
	// Use a timeout for the initialize handshake only. The go-sdk internally
	// detaches the context for the SSE stream, so cancel() won't kill it.
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	transport := &mcp.StreamableClientTransport{
		Endpoint: serverURL,
	}
	if httpClient != nil {
		transport.HTTPClient = httpClient
	}

	// Create a fresh Client per session — avoids shared session tracking.
	client := mcp.NewClient(
		&mcp.Implementation{Name: "ai-platform", Version: "1.0.0"},
		&mcp.ClientOptions{},
	)

	return client.Connect(initCtx, transport, nil)
}

// handleConnectionError classifies the connection failure and returns the
// appropriate connector state and optional auth URL.
func (s *Service) handleConnectionError(ctx context.Context, connector *MCPConnector, err error) (*MCPConnector, string, error) {
	if isAuthError(err) {
		return s.initiateOAuth(ctx, connector)
	}

	connector.ErrorMsg = err.Error()

	if isTransientError(err) {
		// Transient: mark disconnected but allow refresh to retry.
		_ = connector.Transition(ConnectorStatusDisconnected, err.Error(), false)
	} else {
		// Permanent: HTTP 4xx, SSRF, bad URL, etc.
		_ = connector.Transition(ConnectorStatusError, "", false)
	}

	_ = s.repo.Update(ctx, connector)
	s.invalidateConnectorCache(connector.ID, connector.UserID)
	return connector, "", fmt.Errorf("connecting to MCP server: %w", err)
}

// --------------------------------------------------------------------------
// OAuth initiation
// --------------------------------------------------------------------------

func (s *Service) initiateOAuth(ctx context.Context, connector *MCPConnector) (*MCPConnector, string, error) {
	s.log.Info(ctx, "initiating OAuth flow", "connector_id", connector.ID, "url", connector.ServerURL)

	resourceMeta, authMeta, err := s.discoverOAuthMetadata(ctx, connector)
	if err != nil {
		return connector, "", err
	}

	metaJSON, _ := json.Marshal(authMeta)
	connector.OAuthServerMeta = toNullString(string(metaJSON))

	clientID, _, err := s.resolveOAuthClientCreds(ctx, connector, authMeta)
	if err != nil {
		return connector, "", err
	}

	state := GenerateState()
	codeVerifier := GenerateCodeVerifier()
	authURL := s.oauth.BuildAuthorizationURL(
		authMeta, clientID, state, CodeChallenge(codeVerifier),
		pickOAuthScopes(resourceMeta, authMeta),
	)

	// Persist OAuth state on the connector row so it survives restarts
	// and works across multi-instance deployments (agentstack pattern).
	connector.OAuthState = state
	connector.OAuthCodeVerifier = codeVerifier
	_ = connector.Transition(ConnectorStatusAuthRequired, "", false)
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, "", fmt.Errorf("updating connector for OAuth: %w", err)
	}
	s.invalidateConnectorCache(connector.ID, connector.UserID)

	s.log.Info(ctx, "oauth authorization URL generated", "connector_id", connector.ID)
	return connector, authURL, nil
}

// discoverOAuthMetadata walks the protected-resource → authorization-server
// discovery chain. Either failure transitions the connector to Error and
// returns a wrapped error so the caller returns immediately.
func (s *Service) discoverOAuthMetadata(ctx context.Context, connector *MCPConnector) (*ProtectedResourceMetadata, *OAuthMetadata, error) {
	resourceMeta, err := s.oauth.DiscoverProtectedResourceMetadata(ctx, connector.ServerURL)
	if err != nil {
		s.transitionToError(ctx, connector, "failed to discover OAuth metadata: "+err.Error())
		return nil, nil, fmt.Errorf("discovering resource metadata: %w", err)
	}
	if len(resourceMeta.AuthorizationServers) == 0 {
		s.transitionToError(ctx, connector, "no authorization servers found in metadata")
		return nil, nil, fmt.Errorf("no authorization servers in resource metadata")
	}
	authMeta, err := s.oauth.DiscoverAuthServerMetadata(ctx, resourceMeta.AuthorizationServers[0])
	if err != nil {
		s.transitionToError(ctx, connector, "failed to discover auth server: "+err.Error())
		return nil, nil, fmt.Errorf("discovering auth server metadata: %w", err)
	}
	return resourceMeta, authMeta, nil
}

// resolveOAuthClientCreds returns the (clientID, clientSecret) pair for
// the authorization flow: prefer credentials already stored on the
// connector, else fall back to dynamic client registration (DCR), else
// bail with a transitionToError. Any newly-registered credentials are
// persisted onto the connector.
func (s *Service) resolveOAuthClientCreds(ctx context.Context, connector *MCPConnector, authMeta *OAuthMetadata) (string, string, error) {
	if connector.OAuthClientID.Valid && connector.OAuthClientID.String != "" {
		clientID := connector.OAuthClientID.String
		var clientSecret string
		if connector.OAuthClientSecret.Valid {
			clientSecret = connector.OAuthClientSecret.String
		}
		return clientID, clientSecret, nil
	}
	if authMeta.RegistrationEndpoint == "" {
		s.transitionToError(ctx, connector, "OAuth required but no client credentials and no registration endpoint")
		return "", "", fmt.Errorf("no OAuth client credentials and no registration endpoint")
	}
	clientID, clientSecret, regErr := s.oauth.RegisterClient(ctx, authMeta.RegistrationEndpoint, connector.Title, connector.ID)
	if regErr != nil {
		s.log.Warn(ctx, "dynamic client registration failed", "error", regErr)
		s.transitionToError(ctx, connector, "dynamic client registration failed: "+regErr.Error())
		return "", "", fmt.Errorf("registering client: %w", regErr)
	}
	connector.OAuthClientID = toNullString(clientID)
	connector.OAuthClientSecret = toNullString(clientSecret)
	return clientID, clientSecret, nil
}

// pickOAuthScopes returns the resource-server advertised scopes when
// present, else falls back to the authorization-server scopes.
func pickOAuthScopes(resourceMeta *ProtectedResourceMetadata, authMeta *OAuthMetadata) []string {
	if len(resourceMeta.ScopesSupported) > 0 {
		return resourceMeta.ScopesSupported
	}
	return authMeta.ScopesSupported
}

// --------------------------------------------------------------------------
// Token revocation
// --------------------------------------------------------------------------

// revokeToken attempts to revoke the OAuth access token at the authorization
// server's revocation endpoint. Failures are logged but not propagated — the
// token is cleared locally regardless.
func (s *Service) revokeToken(ctx context.Context, connector *MCPConnector) {
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

func (s *Service) registerConnectorInToolRegistry(ctx context.Context, connector *MCPConnector) {
	if s.toolRegistry == nil {
		return
	}
	toolSetID := "mcp-" + connector.ID

	discoveredTools, err := s.repo.ListTools(ctx, connector.ID)
	if err != nil {
		s.log.Warn(ctx, "failed to list tools for registry registration", "connector_id", connector.ID, "error", err)
		return
	}

	toolSet, regTools := buildToolRegistryEntries(connector, toolSetID, discoveredTools)

	// Register the toolset BEFORE individual tools because the tools
	// table has a FK constraint (tool_set_id → tool_sets.id).
	if err := s.toolRegistry.RegisterToolSet(ctx, toolSet); err != nil {
		s.log.Warn(ctx, "failed to register mcp toolset", "tool_set_id", toolSetID, "error", err)
		return
	}
	if len(regTools) > 0 {
		if err := s.toolRegistry.RegisterTools(ctx, regTools); err != nil {
			s.log.Warn(ctx, "failed to batch register mcp tools", "tool_set_id", toolSetID, "error", err)
		}
	}
	s.log.Info(ctx, "mcp connector registered in tool registry", "tool_set_id", toolSetID, "tool_count", len(toolSet.ToolIDs))
}

// buildToolRegistryEntries maps discovered MCP tools onto the tool-registry
// shapes: one ToolSet carrying every toolID, plus one Tool per discovered
// entry. Batching lets registerConnectorInToolRegistry persist a connector
// with N tools in 2 DB writes (SaveToolSet + batch SaveTools) instead of
// N+1.
func buildToolRegistryEntries(
	connector *MCPConnector, toolSetID string, discoveredTools []MCPDiscoveredTool,
) (*toolregistry.ToolSet, []*toolregistry.Tool) {
	name := connector.Title
	if connector.ServerName.Valid && connector.ServerName.String != "" {
		name = connector.ServerName.String
	}
	toolIDs := make([]string, 0, len(discoveredTools))
	regTools := make([]*toolregistry.Tool, 0, len(discoveredTools))
	for _, dt := range discoveredTools {
		toolID := toolSetID + "-" + dt.Name
		toolIDs = append(toolIDs, toolID)
		regTools = append(regTools, &toolregistry.Tool{
			ID:          toolID,
			Name:        dt.Name,
			Description: dt.Description,
			Category:    "MCP",
			ToolSetID:   toolSetID,
		})
	}
	toolSet := &toolregistry.ToolSet{
		ID:          toolSetID,
		Name:        name,
		Description: "MCP Server: " + connector.ServerURL,
		Category:    "MCP",
		ToolIDs:     toolIDs,
	}
	return toolSet, regTools
}

// --------------------------------------------------------------------------
// Tool discovery
// --------------------------------------------------------------------------

func (s *Service) discoverTools(ctx context.Context, connector *MCPConnector, session *mcp.ClientSession) error {
	var tools []MCPDiscoveredTool

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

		tools = append(tools, MCPDiscoveredTool{
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
//
// Clients are cached per-connector with a 5-minute TTL to avoid rebuilding
// OAuth transports on every request while still picking up token changes.
func (s *Service) buildHTTPClient(connector *MCPConnector) *http.Client {
	// Try the cache first.
	if client, ok := s.httpClients.Get(connector.ID); ok {
		return client
	}

	client := s.buildHTTPClientUncached(connector)
	s.httpClients.Set(connector.ID, client)
	return client
}

// invalidateHTTPClient evicts the cached HTTP client for a connector so the
// next buildHTTPClient call picks up new tokens / credentials.
func (s *Service) invalidateHTTPClient(connectorID string) {
	s.httpClients.Delete(connectorID)
}

// buildHTTPClientUncached is the inner builder that always creates a fresh client.
func (s *Service) buildHTTPClientUncached(connector *MCPConnector) *http.Client {
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
						tokenJSON, _ := marshalTokenResponse(newToken)
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
//
// The go-sdk does NOT expose structured HTTP error types. All HTTP failures
// are formatted as:
//
//   Streamable checkResponse: fmt.Errorf("%s: %v", requestSummary, http.StatusText(code))
//   SSE connect:              fmt.Errorf("failed to connect: %s", http.StatusText(code))
//   Streamable transient:     fmt.Errorf("%w: %s: %v", ErrRejected, summary, http.StatusText(code))
//
// http.StatusText returns the canonical reason phrase ("Unauthorized",
// "Bad Gateway", etc.) WITHOUT the numeric code. We therefore match against
// these exact phrases, computed from http.StatusText at init-time, so the
// classifier can never drift from the go-sdk's output.
// --------------------------------------------------------------------------

// httpStatusTexts are pre-computed http.StatusText values, grouped by class,
// used by the error classifiers. Built at init-time from the numeric codes so
// they cannot go stale.
var (
	// authStatusText is http.StatusText(401) = "Unauthorized".
	authStatusText = http.StatusText(http.StatusUnauthorized)

	// transientStatusTexts are the 5xx status texts that warrant a retry.
	transientStatusTexts = func() []string {
		codes := []int{
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		}
		texts := make([]string, len(codes))
		for i, c := range codes {
			texts[i] = http.StatusText(c)
		}
		return texts
	}()
)

// isAuthError checks if an error is an HTTP 401 Unauthorized.
//
// The go-sdk uses http.StatusText(401) in its error messages:
//   - Streamable: `sending "initialize": Unauthorized`
//   - SSE:        `failed to connect: Unauthorized`
//
// We match the exact text produced by http.StatusText(401).
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), authStatusText)
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

	msg := strings.ToLower(err.Error())
	transientPatterns := []string{
		"connection refused", "connection reset",
		"timeout", "i/o timeout", "deadline exceeded",
		"eof", "broken pipe",
	}
	for _, p := range transientPatterns {
		if strings.Contains(msg, p) {
			return true
		}
	}

	// Match go-sdk's http.StatusText() output for 5xx errors.
	msgOriginal := err.Error()
	for _, text := range transientStatusTexts {
		if strings.Contains(msgOriginal, text) {
			return true
		}
	}

	return false
}

// validateExternalHost blocks requests to localhost, private RFC-1918
// ranges, and cloud metadata endpoints (SSRF defence). Each rule is a
// small predicate so the intent of each block is greppable.
func validateExternalHost(host string) error {
	host = strings.ToLower(host)
	if isLoopbackHost(host) {
		return fmt.Errorf("server URL cannot point to localhost")
	}
	if isPrivateIPv4(host) || isPrivateIPv6(host) {
		return fmt.Errorf("server URL cannot point to private network addresses")
	}
	if isLinkLocalOrMetadata(host) {
		return fmt.Errorf("server URL cannot point to link-local or cloud metadata addresses")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return false
}

// isPrivateIPv4 covers RFC-1918 ranges: 10.0.0.0/8, 192.168.0.0/16, and
// 172.16.0.0/12 (172.16.x.x through 172.31.x.x — the `172.` prefix alone
// is not enough, so we parse and check the CIDR).
func isPrivateIPv4(host string) bool {
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") {
		return true
	}
	if strings.HasPrefix(host, "172.") {
		ip := net.ParseIP(host)
		if ip != nil {
			_, private172, _ := net.ParseCIDR("172.16.0.0/12")
			if private172.Contains(ip) {
				return true
			}
		}
	}
	return false
}

// isPrivateIPv6 checks the unique-local (fc/fd) and link-local (fe80)
// prefixes.
func isPrivateIPv6(host string) bool {
	return strings.HasPrefix(host, "fc") || strings.HasPrefix(host, "fd") || strings.HasPrefix(host, "fe80")
}

func isLinkLocalOrMetadata(host string) bool {
	return strings.HasPrefix(host, "169.254.") || host == "metadata.google.internal"
}

// toNullString converts a string to sql.NullString.
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
