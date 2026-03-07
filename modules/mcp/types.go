package mcp

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ConnectorStatus represents the lifecycle state of an MCP connector.
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

// MCPConnector represents a user-registered remote MCP server.
type MCPConnector struct {
	ID        string          `json:"id" db:"id"`
	UserID    string          `json:"user_id" db:"user_id"`
	Title     string          `json:"title" db:"title"`
	ServerURL string          `json:"server_url" db:"server_url"`
	Status    ConnectorStatus `json:"status" db:"status"`
	ErrorMsg  string          `json:"error_msg,omitempty" db:"error_msg"`

	// Disconnect tracking — distinguishes transient failures from permanent ones.
	DisconnectReason    string `json:"disconnect_reason,omitempty" db:"disconnect_reason"`
	DisconnectPermanent bool   `json:"disconnect_permanent,omitempty" db:"disconnect_permanent"`

	// OAuth fields (nullable — set only if the server requires auth)
	OAuthClientID     sql.NullString `json:"-" db:"oauth_client_id"`
	OAuthClientSecret sql.NullString `json:"-" db:"oauth_client_secret"` // encrypted at rest
	OAuthTokenJSON    sql.NullString `json:"-" db:"oauth_token_json"`    // encrypted at rest
	OAuthServerMeta   sql.NullString `json:"-" db:"oauth_server_meta"`   // JSON: cached RFC 8414 metadata

	// OAuth flow state — persisted so in-progress flows survive server restarts.
	OAuthState        string `json:"-" db:"oauth_state"`
	OAuthCodeVerifier string `json:"-" db:"oauth_code_verifier"`

	// Discovered server metadata (populated after successful connection)
	ServerName    sql.NullString `json:"server_name,omitempty" db:"server_name"`
	ServerVersion sql.NullString `json:"server_version,omitempty" db:"server_version"`
	Capabilities  sql.NullString `json:"capabilities,omitempty" db:"capabilities"` // JSON

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Transition validates and applies a state change. It enforces:
//   - "created" is the initial state and cannot be transitioned to.
//   - moving to "disconnected" requires a reason and a permanence flag.
//   - moving to any other state clears the disconnect fields.
func (c *MCPConnector) Transition(status ConnectorStatus, reason string, permanent bool) error {
	if status == ConnectorStatusCreated {
		return fmt.Errorf("cannot transition back to created state")
	}
	if status == ConnectorStatusDisconnected {
		if reason == "" {
			return fmt.Errorf("disconnect reason is required when transitioning to disconnected")
		}
	} else {
		// Clear disconnect fields when moving to a non-disconnected state.
		reason = ""
		permanent = false
	}
	c.Status = status
	c.DisconnectReason = reason
	c.DisconnectPermanent = permanent
	return nil
}

// MCPDiscoveredTool represents a tool discovered from a connected MCP server.
type MCPDiscoveredTool struct {
	ID          string    `json:"id" db:"id"`
	ConnectorID string    `json:"connector_id" db:"connector_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	InputSchema string    `json:"input_schema" db:"input_schema"` // JSON Schema
	Annotations string    `json:"annotations" db:"annotations"`   // JSON: MCP tool annotations
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// MCPConnectorRepository defines persistence for MCP connectors.
type MCPConnectorRepository interface {
	// Connector CRUD
	Create(ctx context.Context, connector *MCPConnector) error
	GetByID(ctx context.Context, id string) (*MCPConnector, error)
	GetByOAuthState(ctx context.Context, state string) (*MCPConnector, error)
	ListByUser(ctx context.Context, userID string) ([]MCPConnector, error)
	CountByUser(ctx context.Context, userID string) (int, error)
	Update(ctx context.Context, connector *MCPConnector) error
	Delete(ctx context.Context, id string) error

	// Tool operations
	UpsertTools(ctx context.Context, connectorID string, tools []MCPDiscoveredTool) error
	ListTools(ctx context.Context, connectorID string) ([]MCPDiscoveredTool, error)
	DeleteToolsByConnector(ctx context.Context, connectorID string) error
	// CountToolsByConnectors returns a map of connector ID → tool count for a
	// batch of connectors. Avoids N+1 queries when listing connectors.
	CountToolsByConnectors(ctx context.Context, connectorIDs []string) (map[string]int, error)
}
