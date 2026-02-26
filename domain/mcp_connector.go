package domain

import (
	"context"
	"database/sql"
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

	// OAuth fields (nullable — set only if the server requires auth)
	OAuthClientID     sql.NullString `json:"-" db:"oauth_client_id"`
	OAuthClientSecret sql.NullString `json:"-" db:"oauth_client_secret"` // encrypted at rest
	OAuthTokenJSON    sql.NullString `json:"-" db:"oauth_token_json"`    // encrypted at rest
	OAuthServerMeta   sql.NullString `json:"-" db:"oauth_server_meta"`   // JSON: cached RFC 8414 metadata

	// Discovered server metadata (populated after successful connection)
	ServerName    sql.NullString `json:"server_name,omitempty" db:"server_name"`
	ServerVersion sql.NullString `json:"server_version,omitempty" db:"server_version"`
	Capabilities  sql.NullString `json:"capabilities,omitempty" db:"capabilities"` // JSON

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
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
	ListByUser(ctx context.Context, userID string) ([]MCPConnector, error)
	Update(ctx context.Context, connector *MCPConnector) error
	Delete(ctx context.Context, id string) error

	// Tool operations
	UpsertTools(ctx context.Context, connectorID string, tools []MCPDiscoveredTool) error
	ListTools(ctx context.Context, connectorID string) ([]MCPDiscoveredTool, error)
	DeleteToolsByConnector(ctx context.Context, connectorID string) error
}
