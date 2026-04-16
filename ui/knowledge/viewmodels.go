package knowledge

import "github.com/google/uuid"

// WorkspaceViewModel contains the workspace fields needed by templates.
// Decouples the presentation layer from the knowledge module's domain types.
type WorkspaceViewModel struct {
	ID          uuid.UUID
	Name        string
	Description *string
}
