package permissions

import (
	"aicompass/modules/knowledge/tenant"
	"context"

	"github.com/google/uuid"
)

// Workspace models the data required by the permissions module for a workspace.
type Workspace struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Slug        string    `json:"slug"`
	IsActive    bool      `json:"is_active"`
}

// WorkspaceList holds a paginated list of Workspace objects.
type WorkspaceList struct {
	Items []Workspace `json:"items"`
}

// WorkspaceProvider abstracts the dependency on the knowledge module.
type WorkspaceProvider interface {
	// ListWorkspaces retrieves a list of workspaces accessible in the given tenant context.
	ListWorkspaces(ctx context.Context, tCtx *tenant.TenantContext, page, pageSize int) (*WorkspaceList, error)
}
