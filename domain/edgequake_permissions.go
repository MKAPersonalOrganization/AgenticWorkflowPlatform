package domain

import (
	"time"

	"github.com/google/uuid"
)

// TenantContext carries the active multitenancy state for a request.
type TenantContext struct {
	TenantID    *uuid.UUID `json:"tenant_id"`
	WorkspaceID *uuid.UUID `json:"workspace_id,omitempty"`
	UserID      string     `json:"user_id"`
	Role        string     `json:"role"`
}

// EdgequakeRole represents a user's role in a tenant or workspace.
type EdgequakeRole string

const (
	RoleAdmin    EdgequakeRole = "admin"
	RoleUser     EdgequakeRole = "user"
	RoleReadonly EdgequakeRole = "readonly"
)

// EdgequakeTenant represents a top-level organization in Edgequake.
type EdgequakeTenant struct {
	ID            uuid.UUID      `json:"tenant_id"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	IsActive      bool           `json:"is_active"`
	Plan          string         `json:"plan"`
	MaxWorkspaces int            `json:"max_workspaces"`
	MaxUsers      int            `json:"max_users"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	Metadata      map[string]any `json:"metadata"`
}

// EdgequakeWorkspace represents a knowledge base within a tenant.
type EdgequakeWorkspace struct {
	ID        uuid.UUID      `json:"workspace_id"`
	TenantID  uuid.UUID      `json:"tenant_id"`
	Name      string         `json:"name"`
	Slug      string         `json:"slug"`
	IsActive  bool           `json:"is_active"`
	Metadata  map[string]any `json:"metadata"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// EdgequakeMembership links a user to a tenant and optionally a specific workspace.
type EdgequakeMembership struct {
	ID          uuid.UUID     `json:"membership_id"`
	UserID      string        `json:"user_id"` // Matches domain.User.ID (Kratos ID)
	TenantID    uuid.UUID     `json:"tenant_id"`
	WorkspaceID *uuid.UUID    `json:"workspace_id,omitempty"` // nil means access to all workspaces in tenant
	Role        EdgequakeRole `json:"role"`
	IsActive    bool          `json:"is_active"`
	JoinedAt    time.Time     `json:"joined_at"`
}

// EdgequakePermissionRepository defines the storage interface for permission entities.
type EdgequakePermissionRepository interface {
	// Tenant operations
	SaveTenant(tenant *EdgequakeTenant) error
	GetTenant(id uuid.UUID) (*EdgequakeTenant, error)

	// Workspace operations
	SaveWorkspace(workspace *EdgequakeWorkspace) error
	GetWorkspace(id uuid.UUID) (*EdgequakeWorkspace, error)

	// Membership operations
	SaveMembership(membership *EdgequakeMembership) error
	ListMembershipsByUser(userID string) ([]*EdgequakeMembership, error)

	// Admin/General operations
	ListTenants() ([]*EdgequakeTenant, error)
}
