package resources

import (
	"time"

	"github.com/google/uuid"
)

// ResourceSource defines the source of the resource.
type ResourceSource string

const (
	SourcePublic  ResourceSource = "public"
	SourcePrivate ResourceSource = "private"
)

// Resource represents a document in the library.
type Resource struct {
	ID        uuid.UUID      `json:"id" db:"resource_id"`
	UserID    *string        `json:"user_id,omitempty" db:"user_id"` // nil for public resources
	Title     string         `json:"title" db:"title"`
	FileType  string         `json:"file_type" db:"file_type"`
	FilePath  string         `json:"file_path" db:"file_path"`
	Source    ResourceSource `json:"source" db:"source"`
	CreatedAt time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt time.Time      `json:"updated_at" db:"updated_at"`
}
