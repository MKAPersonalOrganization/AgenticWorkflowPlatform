# ADR 010: MCP Connectors System for User-Registered Remote Tool Servers

## Status
Accepted

## Context
The existing plugin system (ADR 008) provides a powerful, modular architecture for **built-in** tools — those shipped with the application and registered at compile time via `PluginProvider` implementations. However, it offers no mechanism for **end-users** to bring their own tools at runtime.

The Model Context Protocol (MCP) has emerged as an open standard for connecting AI systems to external tool servers. Users increasingly operate personal MCP servers (Notion, GitHub, custom internal tools) exposed over Streamable HTTP and expect to plug them directly into their agent workspace without code changes or redeployment.

Key requirements driving this decision:

- **Runtime Registration:** Users must be able to add, connect, disconnect, and remove MCP servers entirely through the UI.
- **Tool Discovery:** Once connected, the tools advertised by the remote server must be automatically discovered and surfaced identically to built-in plugins (appear in the sidebar, respect enable/disable per-agent).
- **Lifecycle Ephemerality:** MCP tool availability must be tightly coupled to connection state — tools appear on connect, vanish on disconnect or delete, and stale entries are cleaned on application restart.
- **Security:** OAuth 2.1 with PKCE must be supported for servers that require authorization.

## Decision
**Introduce a `services/plugindiscovery` service that manages the full lifecycle of user-registered MCP connectors, bridging them into the existing plugin and permissions systems as ephemeral toolsets.**

### 1. Domain Model
A new `MCPConnector` aggregate is added to `domain/`:

```go
type MCPConnector struct {
    ID            string
    UserID        string
    Name          string
    URL           string
    AuthType      string       // "none" | "oauth"
    Status        string       // "disconnected" | "connected" | "error"
    Tools         []MCPTool
    OAuthConfig   *MCPOAuthConfig
    OAuthToken    *MCPOAuthToken
}
```

Each connector owns zero-or-more `MCPTool` entries discovered via `tools/list` after a successful MCP handshake.

### 2. Service Architecture (`services/plugindiscovery`)
The service is composed of five focused files:

| File | Responsibility |
|---|---|
| `service.go` | CRUD orchestration, connect/disconnect lifecycle, tool-registry bridge |
| `session_manager.go` | MCP client session pool (Streamable HTTP transport) |
| `oauth.go` | OAuth 2.1 + PKCE flow (authorization URL generation, token exchange, refresh) |
| `tool_resolver.go` | Adapts MCP tools into `agent.ExecutableTool` for the runner pipeline |
| `mcp_plugin_provider.go` | Implements `PluginProvider`-style formatting so connectors render in the sidebar |

### 3. Tool Registry Integration
On **connect**, the service registers the connector as a `ToolSet` (ID: `mcp-{connectorID}`) and each discovered tool as a `Tool` in the existing `toolregistry.Registry`. Registration order respects the foreign-key constraint: toolset first, then child tools.

On **disconnect** or **delete**, the service calls `UnregisterToolSet` and `UnregisterTool` to remove all entries from both the database and in-memory cache.

On **application restart**, `toolregistry.Initialize()` scans for any `mcp-` prefixed entries that survived a previous unclean shutdown and deletes them, ensuring no stale phantom plugins persist.

### 4. Agent Pipeline Integration
The `RegistryAgentProvider.GetConfig()` method in the runner adapter is extended with a second resolution pass:

1. **Built-in tools** — resolved via `factory.CreateExecutableToolsFromSet()` (skipping `mcp-` prefixed sets).
2. **MCP tools** — resolved via `mcpService.ResolveEnabledTools()`, which:
   - Iterates all connected connectors for the user.
   - Filters tools through the same `permissions.GetEnabledToolSets()` callbacks used by built-in plugins.
   - Wraps each enabled tool in an `mcpRemoteTool` adapter that delegates `Execute()` to the live MCP session via `CallTool`.

This ensures MCP tools participate in the identical enable/disable permission model as built-in toolsets, with zero special-casing in the permission layer.

### 5. Frontend Reactivity
Two Alpine.js components collaborate:

- **`mcp-connectors.js`** — Manages the connector CRUD UI (add/connect/disconnect/delete dialogs). After every state-changing operation, dispatches a `mcp-state-changed` custom DOM event.
- **`tools-sidebar.js`** — Listens for `mcp-state-changed` and calls `fetchPlugins()` to refresh the plugin list, ensuring the sidebar immediately reflects connection state changes without a full page reload.

### 6. OAuth 2.1 Flow
For connectors with `authType: "oauth"`:

1. The service discovers the server's OAuth metadata via `/.well-known/oauth-authorization-server`.
2. Dynamically registers as a client if no `client_id` is stored.
3. Generates a PKCE `code_verifier` / `code_challenge` pair.
4. Redirects the user to the authorization endpoint.
5. On callback, exchanges the code for tokens and persists them encrypted in the database.
6. Tokens are attached to subsequent MCP session transports automatically.

### 7. Persistence
A migration (v1.23) adds two tables, with subsequent migrations for production hardening:

- `mcp_connectors` — Connector metadata, OAuth config/tokens (JSONB), status, disconnect tracking.
- `mcp_connector_tools` — Discovered tools with name, description, and JSON Schema input definition.

Both are scoped to `user_id` with cascading deletes.

Additional migrations:
- **v1.24** — Adds `disconnect_reason` and `disconnect_permanent` columns for state-machine tracking.
- **v1.25** — Adds `oauth_state` / `oauth_code_verifier` columns (DB-persisted OAuth flow survives restarts), unique index on `(user_id, server_url)`, partial indexes for refreshable connectors and OAuth state lookups.

### 8. Production Hardening (learned from BeeAI/agentstack reference)

| Pattern | Implementation |
|---|---|
| **State Machine** | `MCPConnector.Transition()` validates state changes; `Refreshable()` determines health-check eligibility |
| **Probe with Retries** | `probeConnector()` retries up to 3 times with exponential backoff on transient errors |
| **Error Classification** | `isTransientError()` (DNS, timeouts, 5xx), `isAuthError()` (401), `isHTTPClientError()` (4xx) route to different recovery paths |
| **SSRF Protection** | `validateExternalHost()` blocks localhost, RFC-1918, link-local, cloud metadata |
| **Token Auto-Refresh** | `AutoRefreshTransport` proactively refreshes tokens before expiry and retries on 401, persisting new tokens to DB |
| **Token Revocation** | `RevokeToken()` (RFC 7009) called on disconnect/delete; best-effort with local token clear regardless |
| **DB-Persisted OAuth State** | OAuth `state` + `code_verifier` stored on the connector row; survives restarts and works in multi-instance deployments |
| **Background Health Checks** | `StartHealthCheck()` runs a 30s ticker; `RefreshAllConnectors()` uses bounded concurrency (10 goroutines) with 60s per-connector timeout |
| **Response Size Limits** | `io.LimitReader` on all OAuth metadata (1 MiB) and token (256 KiB) responses |
| **Per-User Limits** | `maxConnectorsPerUser = 20` enforced at creation |
| **Unique Constraint** | `(user_id, server_url)` prevents duplicate registrations |
| **Disconnect State Validation** | `DisconnectConnector` validates the connector is in a disconnectable state before proceeding |
| **OAuth Error Handling** | Callback handler processes `?error=` and `?error_description=` from OAuth providers |
| **Request Body Limits** | `http.MaxBytesReader` (1 MiB) on create endpoint |
| **Typed Service Errors** | Sentinel errors (`ErrConnectorNotFound`, `ErrDuplicateConnector`, etc.) checked via `errors.Is()` for deterministic HTTP status mapping |
| **Structured JSON Errors** | `writeJSONError()` + `mapServiceError()` return `{"error":"..."}` with correct status codes (404, 409, 403, 400, 500) |
| **OAuth Metadata Cache** | In-memory TTL cache (`ttlCache[T]`) — 10 min for resource/auth metadata, 24 h for client registration — avoids redundant network round-trips |
| **Access Token Injection** | `ConnectConnector(ctx, id, accessToken)` accepts optional pre-authorized Bearer token, falls through to OAuth on 401 |
| **Disconnect Returns Entity** | `DisconnectConnector` returns the updated `*MCPConnector` and clears pending OAuth state |

## Consequences

**Positive:**
- **Zero-Code Extensibility:** Users can integrate any MCP-compliant server (Notion, GitHub, Slack, custom internal tools) entirely through the UI without backend changes or redeployment.
- **Unified Permission Model:** MCP tools reuse the existing per-agent enable/disable infrastructure identically to built-in plugins, avoiding a parallel permission system.
- **Clean Lifecycle:** The ephemeral registration strategy (connect→register, disconnect→unregister, restart→clean) guarantees no ghost tools linger in the registry.
- **Standard Protocol:** Building on MCP means compatibility with any server implementing the open standard, future-proofing against vendor-specific integrations.

**Negative:**
- **Runtime Dependency:** MCP tool availability depends on the external server being reachable. Network failures or server downtime will surface as tool execution errors at conversation time.
- **Session Management Overhead:** Each connected MCP server maintains a persistent Streamable HTTP session. At scale, the session pool requires monitoring and potential connection-count limits.
- **Security Surface:** Allowing user-registered arbitrary URLs introduces a server-side request surface that must be hardened (URL validation, timeout enforcement, response-size limits).

## Constraints
1. MCP connectors MUST only support Streamable HTTP transport. Legacy SSE or stdio transports are explicitly out of scope.
2. Tool registration MUST always create the parent `ToolSet` before child `Tool` entries to satisfy the foreign-key constraint (`tools.tool_set_id → tool_sets.id`).
3. The `mcp-` prefix on toolset/tool IDs is a reserved namespace. Built-in plugins MUST NOT use this prefix.
4. OAuth tokens MUST be stored encrypted at rest and scoped strictly to the owning user.
5. `ResolveEnabledTools` MUST filter through the same permission callbacks as built-in tools — no bypass path.
