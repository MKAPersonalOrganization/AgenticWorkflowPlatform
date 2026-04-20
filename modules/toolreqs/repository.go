package toolreqs

import "context"

// =============================================================================
// Repository — persistence contract owned by toolreqs
// =============================================================================

// Repository defines the storage interface for tool set and individual tool
// permission data. Implementations must handle CRUD for both [Permission]
// (individual tool) and [ToolSetPermission] (tool set) records.
//
// The PostgreSQL implementation is [PostgresRepository].
// The in-memory implementation for tests lives in toolreqs/mocks.
type Repository interface {
	// ── Individual Tool Permissions within Tool Sets ──

	// GetPermission retrieves a specific agent-tool permission.
	GetPermission(ctx context.Context, userID UserID, agentID AgentID, toolSetID ToolSetID, toolID ToolID) (*Permission, error)

	// ListPermissions returns all individual tool permissions for an agent within a tool set.
	ListPermissions(ctx context.Context, userID UserID, agentID AgentID, toolSetID ToolSetID) ([]Permission, error)

	// SavePermission saves or updates an individual tool permission.
	SavePermission(ctx context.Context, permission *Permission) error

	// DeletePermission removes a specific agent-tool permission.
	DeletePermission(ctx context.Context, userID UserID, agentID AgentID, toolSetID ToolSetID, toolID ToolID) error

	// ── Tool Set Permissions ──

	// GetToolSetPermission retrieves a specific agent-toolset permission.
	GetToolSetPermission(ctx context.Context, userID UserID, agentID AgentID, toolSetID ToolSetID) (*ToolSetPermission, error)

	// ListToolSetPermissions returns all tool set permissions for an agent.
	ListToolSetPermissions(ctx context.Context, userID UserID, agentID AgentID) ([]ToolSetPermission, error)

	// SaveToolSetPermission saves or updates a tool set permission.
	SaveToolSetPermission(ctx context.Context, permission *ToolSetPermission) error

	// DeleteToolSetPermission removes a specific agent-toolset permission.
	DeleteToolSetPermission(ctx context.Context, userID UserID, agentID AgentID, toolSetID ToolSetID) error
}
