package shared

import (
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
