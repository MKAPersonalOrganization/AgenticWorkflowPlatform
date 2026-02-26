# MCP User-Registered Plugins — Implementation Plan

## Overview

Enable users to register their own MCP (Model Context Protocol) tool servers by pasting a **Streamable HTTP** URL into the UI. The system connects, discovers tools via MCP, and registers them as first-class plugins alongside built-in plugins (calculator, memory, etc.). If the remote server requires authentication, the user is guided through an **OAuth 2.1 Authorization Code + PKCE** flow automatically.

### Key References
- **Go MCP SDK** (`github.com/modelcontextprotocol/go-sdk/mcp`) — Streamable HTTP client transport, session management, tool discovery
- **AgentStack** — Connector-as-entity pattern, demand/fulfillment model, OAuth proxy, popup-based auth
- **MCP Inspector** — OAuth state machine (discover → register → authorize → exchange), auth debugger, direct/proxy connection modes
- **LibreChat** — Full MCP CRUD builder panel, multi-transport support, domain allowlisting, permission system

---

## Architecture

```
                    ┌──────────────────────────────────────────┐
                    │           Frontend (Templ + Alpine.js)    │
                    │                                          │
                    │  Plugin Sidebar                          │
                    │  ┌────────────────────────────┐          │
                    │  │ [+ Add MCP Server]         │          │
                    │  │                            │          │
                    │  │ Built-in Plugins:          │          │
                    │  │  ✅ Calculator              │          │
                    │  │  ✅ Memory                  │          │
                    │  │  ...                       │          │
                    │  │                            │          │
                    │  │ MCP Servers:               │          │
                    │  │  🟢 Stripe Tools (3 tools) │          │
                    │  │  🟠 GitHub (Auth Required)  │          │
                    │  │  🔴 Slack (Error)           │          │
                    │  └────────────────────────────┘          │
                    └──────────────┬───────────────────────────┘
                                   │ HTTP API
                    ┌──────────────▼───────────────────────────┐
                    │           Backend (Go)                    │
                    │                                          │
                    │  /v1/mcp/connectors     CRUD + connect   │
                    │  /v1/mcp/oauth/callback  OAuth callback  │
                    │                                          │
                    │  ┌─────────────────────┐                 │
                    │  │ MCPDiscoveryService  │                 │
                    │  │  - SessionManager   │──── mcp.Client ──────► Remote MCP Server
                    │  │  - OAuthService     │        │               (Streamable HTTP)
                    │  │  - ToolResolver     │        │
                    │  └─────────────────────┘        │
                    │           │                     │
                    │  ┌────────▼──────────┐   ┌──────▼──────┐
                    │  │  PostgreSQL       │   │ In-Memory   │
                    │  │  - connectors     │   │ Session Pool│
                    │  │  - discovered_tools│   └─────────────┘
                    │  └──────────────────┘                    │
                    └──────────────────────────────────────────┘
```

---

## Phase 1: Core Domain & Infrastructure

### 1.1 Domain Model — `domain/mcp_connector.go`

New domain types:

```go
// ConnectorStatus represents the lifecycle state of an MCP connector
type ConnectorStatus string

const (
    ConnectorStatusCreated      ConnectorStatus = "created"
    ConnectorStatusConnecting   ConnectorStatus = "connecting"
    ConnectorStatusConnected    ConnectorStatus = "connected"
    ConnectorStatusAuthRequired ConnectorStatus = "auth_required"
    ConnectorStatusAuthorizing  ConnectorStatus = "authorizing"
    ConnectorStatusDisconnected ConnectorStatus = "disconnected"
    ConnectorStatusError        ConnectorStatus = "error"
)

// MCPConnector represents a user-registered remote MCP server
type MCPConnector struct {
    ID        string
    UserID    string
    Title     string
    ServerURL string
    Status    ConnectorStatus
    ErrorMsg  string            // populated when Status == error

    // OAuth (nullable — set only if server requires auth)
    OAuthClientID     *string
    OAuthClientSecret *string   // encrypted at rest
    OAuthTokenJSON    *string   // encrypted at rest
    OAuthServerMeta   *string   // JSON: cached RFC 8414 metadata

    // Discovered server metadata (after successful connection)
    ServerName    *string
    ServerVersion *string
    Capabilities  *string       // JSON

    CreatedAt time.Time
    UpdatedAt time.Time
}

// MCPDiscoveredTool represents a tool discovered from a connected MCP server
type MCPDiscoveredTool struct {
    ID            string
    ConnectorID   string
    Name          string
    Description   string
    InputSchema   string        // JSON Schema
    Annotations   string        // JSON: MCP tool annotations
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### 1.2 Repository Interface — `domain/mcp_connector.go`

```go
type MCPConnectorRepository interface {
    Create(ctx context.Context, connector *MCPConnector) error
    GetByID(ctx context.Context, id string) (*MCPConnector, error)
    ListByUser(ctx context.Context, userID string) ([]MCPConnector, error)
    Update(ctx context.Context, connector *MCPConnector) error
    Delete(ctx context.Context, id string) error

    // Tool operations
    UpsertTools(ctx context.Context, connectorID string, tools []MCPDiscoveredTool) error
    ListTools(ctx context.Context, connectorID string) ([]MCPDiscoveredTool, error)
    DeleteToolsByConnector(ctx context.Context, connectorID string) error
}
```

### 1.3 Database Migration

New tables: `mcp_connectors` and `mcp_discovered_tools`.

```sql
CREATE TABLE IF NOT EXISTS mcp_connectors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    title           TEXT NOT NULL,
    server_url      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'created',
    error_msg       TEXT NOT NULL DEFAULT '',

    oauth_client_id      TEXT,
    oauth_client_secret  TEXT,   -- encrypted
    oauth_token_json     TEXT,   -- encrypted
    oauth_server_meta    JSONB,

    server_name     TEXT,
    server_version  TEXT,
    capabilities    JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_mcp_connectors_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_mcp_connectors_user_id ON mcp_connectors(user_id);

CREATE TABLE IF NOT EXISTS mcp_discovered_tools (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connector_id    UUID NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    input_schema    JSONB NOT NULL DEFAULT '{}',
    annotations     JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_mcp_tools_connector FOREIGN KEY (connector_id) REFERENCES mcp_connectors(id) ON DELETE CASCADE
);

CREATE INDEX idx_mcp_discovered_tools_connector ON mcp_discovered_tools(connector_id);
```

### 1.4 PostgreSQL Repository — `infra/mcp_connector_repo.go`

Standard sqlx-based CRUD implementation for the above interface.

---

## Phase 2: MCP Client Service Layer

### 2.1 Session Manager — `services/plugindiscovery/session_manager.go`

Manages a pool of live `mcp.ClientSession` instances (one per connected connector).

```go
type SessionManager struct {
    mu       sync.RWMutex
    client   *mcp.Client
    sessions map[string]*SessionEntry  // connectorID → session
}

type SessionEntry struct {
    Session    *mcp.ClientSession
    Cancel     context.CancelFunc
    Connector  domain.MCPConnector
}
```

Key operations:
- `Connect(ctx, connector, httpClient) → (*mcp.ClientSession, error)` — creates transport + connects
- `Disconnect(connectorID)` — gracefully closes session
- `GetSession(connectorID) → *mcp.ClientSession` — retrieves live session
- `IsConnected(connectorID) → bool`

### 2.2 OAuth Service — `services/plugindiscovery/oauth.go`

Implements the OAuth 2.1 Authorization Code + PKCE flow:

1. **DiscoverMetadata(serverURL)** → fetch `/.well-known/oauth-protected-resource` → `/.well-known/oauth-authorization-server`
2. **RegisterClient(metadata, redirectURI)** → Dynamic Client Registration (RFC 7591)
3. **BuildAuthorizationURL(metadata, clientID, codeVerifier, state)** → authorization URL with PKCE S256 challenge
4. **ExchangeCode(metadata, clientID, clientSecret, code, codeVerifier)** → token response
5. **RefreshToken(metadata, clientID, clientSecret, refreshToken)** → refreshed tokens
6. **BuildAuthHTTPClient(tokenJSON)** → `*http.Client` with Bearer token injection

### 2.3 Discovery Service — `services/plugindiscovery/service.go`

Orchestrates the full lifecycle:

```go
type MCPDiscoveryService struct {
    repo           domain.MCPConnectorRepository
    sessions       *SessionManager
    oauth          *OAuthService
    callbackURL    string  // e.g. https://app.example.com/v1/mcp/oauth/callback
}
```

Key operations:
- **CreateConnector(ctx, userID, url, title)** → creates connector in DB
- **ConnectConnector(ctx, connectorID)** → attempts MCP connection, discovers tools, handles 401→OAuth
- **DisconnectConnector(ctx, connectorID)** → closes session, updates status
- **HandleOAuthCallback(ctx, state, code)** → exchanges code, stores tokens, reconnects
- **GetConnectorTools(ctx, connectorID)** → returns discovered tools
- **DeleteConnector(ctx, connectorID)** → disconnects + deletes from DB

### 2.4 MCP Tool Resolver — `services/plugindiscovery/tool_resolver.go`

Implements `foundation/agent.ToolResolver` to bridge MCP tools into the agent system:

```go
type MCPToolResolver struct {
    sessions *SessionManager
    repo     domain.MCPConnectorRepository
    userID   string
}

func (r *MCPToolResolver) Resolve(ctx context.Context) ([]agent.Tool, error) {
    // 1. List connected connectors for this user
    // 2. For each, get the live session
    // 3. Wrap each MCP tool as an agent.ExecutableTool
    // 4. Return all tools
}
```

Each tool's `Execute()` calls `session.CallTool()` on the remote MCP server.

---

## Phase 3: HTTP API Layer

### 3.1 Handler — `handlers/mcp/handler.go`

```go
type MCPHandler struct {
    service *plugindiscovery.MCPDiscoveryService
}
```

### 3.2 Routes — `handlers/mcp/routes.go`

| Method | Route | Handler | Description |
|--------|-------|---------|-------------|
| `GET` | `/v1/mcp/connectors` | `ListConnectors` | List user's MCP connectors |
| `POST` | `/v1/mcp/connectors` | `CreateConnector` | Add new MCP server URL |
| `GET` | `/v1/mcp/connectors/{id}` | `GetConnector` | Get connector details |
| `DELETE` | `/v1/mcp/connectors/{id}` | `DeleteConnector` | Remove + disconnect |
| `POST` | `/v1/mcp/connectors/{id}/connect` | `Connect` | Initiate MCP connection |
| `POST` | `/v1/mcp/connectors/{id}/disconnect` | `Disconnect` | Close session |
| `GET` | `/v1/mcp/connectors/{id}/tools` | `ListTools` | Discovered tools |
| `GET` | `/v1/mcp/oauth/callback` | `OAuthCallback` | OAuth redirect handler |
| `POST` | `/v1/mcp/connectors/{id}/refresh` | `RefreshTools` | Re-discover tools |

---

## Phase 4: Frontend (Templ + Alpine.js)

### 4.1 "Add MCP Server" Dialog — `components/tools/mcp_dialog.templ`

Modal dialog triggered from plugin sidebar:
- URL input field
- Display name input
- Collapsible "Advanced" section for optional OAuth client_id / client_secret
- Connect button → calls POST /v1/mcp/connectors + POST .../connect
- Loading/error states

### 4.2 MCP Connector Card — `components/tools/mcp_connector_card.templ`

Renders in the plugin sidebar for each registered MCP server:
- Server title + URL
- Status dot (green/yellow/red/orange)
- Tool count badge
- Action buttons: Disconnect, Authorize, Remove

### 4.3 MCP Status Indicator — `components/tools/mcp_status.templ`

Reusable status dot component with tooltip.

### 4.4 OAuth Callback Page — `components/tools/mcp_oauth_callback.templ`

Minimal HTML page served at `/v1/mcp/oauth/callback`:
- Receives authorization code from OAuth provider
- Calls backend to exchange code
- Sends `window.opener.postMessage(...)` to notify main window
- Closes popup

### 4.5 Alpine.js Component — `assets/js/mcp-connectors.js`

```js
Alpine.data('mcpConnectors', () => ({
    connectors: [],
    showAddDialog: false,
    newUrl: '',
    newTitle: '',
    loading: false,
    error: null,

    async init() { await this.fetchConnectors(); },
    async fetchConnectors() { /* GET /v1/mcp/connectors */ },
    async addConnector() { /* POST + connect */ },
    async removeConnector(id) { /* DELETE */ },
    async reconnect(id) { /* POST .../connect */ },
    async disconnect(id) { /* POST .../disconnect */ },
    openOAuthPopup(url) { /* window.open for OAuth */ },
    handleOAuthMessage(event) { /* postMessage listener */ },
}))
```

### 4.6 Plugin Sidebar Integration — `components/tools/tools.templ` (modify)

Add to existing sidebar:
- "Add MCP Server" button in the header area
- New "MCP Servers" section below built-in plugins
- MCP connector cards rendered via `mcp_connector_card.templ`

---

## Phase 5: Agent Integration

### 5.1 Tool Registry Integration — `services/toolregistry/`

- On startup, load all `connected` MCP connectors and register their tools
- Create dynamic `PluginProvider` instances for each connector
- Register with global plugin registry so tools appear in sidebar

### 5.2 Agent Orchestrator Integration — `services/agentorchestrator/`

- Inject `MCPToolResolver` into agent config's `ToolResolvers` slice
- When agent runs, resolver provides MCP tools alongside built-in tools
- Tool execution delegates to `session.CallTool()` on the live MCP session

### 5.3 MCP Plugin Provider — `services/plugindiscovery/mcp_plugin_provider.go`

Dynamic `PluginProvider` that wraps a connected MCP server:
- `ID()` → `"mcp-{connectorID}"`
- `DefaultTools()` → tools from `MCPDiscoveredTool` records
- `CreateExecutable()` → wraps `session.CallTool()`
- `GetUITemplate()` → generic MCP tool result component

---

## Phase 6: Security & Production Hardening

| Concern | Mitigation |
|---------|-----------|
| SSRF | Validate URLs — block private IPs, localhost. Optional admin domain allowlist. |
| Token Storage | OAuth tokens encrypted at rest using app encryption key |
| Transport | Only Streamable HTTP allowed — no stdio command execution |
| User Isolation | Connectors scoped to user. Sessions per-user. No cross-user access. |
| Rate Limiting | Max 10 connectors per user. Max 50 tool calls/min per connector. |
| Token Refresh | Auto-refresh before expiry. Revoke on connector delete. |
| Connection Health | KeepAlive pings. Auto-reconnect on transient failures. |

---

## Implementation Order

```
Phase 1 (Foundation):
  1. Add go-sdk dependency to go.mod
  2. domain/mcp_connector.go (types + repository interface)
  3. Database migration
  4. infra/mcp_connector_repo.go (PostgreSQL repository)

Phase 2 (Service Layer):
  5. services/plugindiscovery/session_manager.go
  6. services/plugindiscovery/oauth.go
  7. services/plugindiscovery/service.go
  8. services/plugindiscovery/tool_resolver.go

Phase 3 (API):
  9.  handlers/mcp/handler.go
  10. handlers/mcp/routes.go

Phase 4 (Frontend):
  11. components/tools/mcp_dialog.templ
  12. components/tools/mcp_connector_card.templ
  13. components/tools/mcp_status.templ
  14. components/tools/mcp_oauth_callback.templ
  15. assets/js/mcp-connectors.js
  16. Modify components/tools/tools.templ (sidebar integration)

Phase 5 (Agent Integration):
  17. services/plugindiscovery/mcp_plugin_provider.go
  18. Wire into toolregistry + agent orchestrator
  19. Wire routes + service into main.go

Phase 6 (Hardening):
  20. URL validation / SSRF protection
  21. Rate limiting
  22. Connection health monitoring
  23. Tests
```

---

## Dependencies

- `github.com/modelcontextprotocol/go-sdk` — MCP client (Streamable HTTP transport, tool discovery, session management)
- `github.com/qmuntal/stateless` — Already in go.mod, used for connector state machine
- `golang.org/x/oauth2` — Already transitive dependency, used for token management
