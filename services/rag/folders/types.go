package folders

import (
	"github.com/google/uuid"
)

// FolderResponse represents a single folder returned by the API.
type FolderResponse struct {
	ID          uuid.UUID  `json:"id"`
	TenantID    uuid.UUID  `json:"tenant_id"`
	WorkspaceID *uuid.UUID `json:"workspace_id,omitempty"`
	Name        string     `json:"name"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	Position    int32      `json:"position"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

// CreateFolderRequest represents the payload to create a new folder.
type CreateFolderRequest struct {
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// UpdateFolderRequest represents the payload to update an existing folder.
type UpdateFolderRequest struct {
	Name     *string    `json:"name,omitempty"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Position *int32     `json:"position,omitempty"`
}
