package filesystem

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ─── Filesystem Types ───────────────────────────────────────────────────────

// FSType represents the backend type of a filesystem.
type FSType string

const (
	// FSTypeLocal is an S3/MinIO-backed virtual filesystem (read-write).
	FSTypeLocal FSType = "local"
	// FSTypeGitHub is a GitHub repository-backed filesystem (read-only).
	FSTypeGitHub FSType = "github"
)

// FileSystem represents a user-owned virtual filesystem.
type FileSystem struct {
	ID          uuid.UUID       `json:"id" db:"filesystem_id"`
	UserID      string          `json:"user_id" db:"user_id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description" db:"description"`
	Type        FSType          `json:"type" db:"fs_type"`
	Config      json.RawMessage `json:"config" db:"config"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// GitHubConfig holds configuration for a GitHub-backed filesystem.
type GitHubConfig struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	// Token is no longer user-supplied; it is resolved at runtime from the
	// github_connections table via OAuth.  Kept for backward-compatibility
	// with any existing rows that stored a PAT directly.
	Token string `json:"token,omitempty"`
}

// ─── Entry Types ────────────────────────────────────────────────────────────

// EntryType represents the type of a filesystem entry.
type EntryType string

const (
	EntryTypeFile      EntryType = "file"
	EntryTypeDirectory EntryType = "directory"
)

// FSEntry represents a file or directory within a filesystem.
type FSEntry struct {
	ID           uuid.UUID `json:"id" db:"entry_id"`
	FilesystemID uuid.UUID `json:"filesystem_id" db:"filesystem_id"`
	ParentPath   string    `json:"parent_path" db:"parent_path"`
	Name         string    `json:"name" db:"name"`
	EntryType    EntryType `json:"entry_type" db:"entry_type"`
	Size         int64     `json:"size" db:"size"`
	ContentType  string    `json:"content_type" db:"content_type"`
	StorageKey   string    `json:"-" db:"storage_key"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// FullPath returns the full path of this entry.
func (e *FSEntry) FullPath() string {
	if e.ParentPath == "/" {
		return "/" + e.Name
	}
	return e.ParentPath + "/" + e.Name
}

// ─── Permissions ────────────────────────────────────────────────────────────

// FSPermission represents agent access to a filesystem.
type FSPermission struct {
	ID           uuid.UUID `json:"id" db:"permission_id"`
	FilesystemID uuid.UUID `json:"filesystem_id" db:"filesystem_id"`
	UserID       string    `json:"user_id" db:"user_id"`
	AgentID      string    `json:"agent_id" db:"agent_id"`
	GrantedAt    time.Time `json:"granted_at" db:"granted_at"`
}

// ─── Request / Response Types ───────────────────────────────────────────────

// CreateFilesystemRequest is the request body for creating a filesystem.
type CreateFilesystemRequest struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        FSType          `json:"type"`
	Config      json.RawMessage `json:"config,omitempty"`
}

// UpdateFilesystemRequest is the request body for updating a filesystem.
type UpdateFilesystemRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// MkdirRequest is the request body for creating a directory.
type MkdirRequest struct {
	Path string `json:"path"`
}

// RenameRequest is the request body for renaming/moving an entry.
type RenameRequest struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
}

// PermissionRequest is the request body for granting agent access.
type PermissionRequest struct {
	AgentID string `json:"agent_id"`
}

// AgentFilesystem represents a filesystem with an indication if a specific agent has access.
type AgentFilesystem struct {
	FileSystem
	Enabled bool `json:"enabled"`
}

// ToggleAgentPermissionRequest is the request body for toggling agent access to a filesystem.
type ToggleAgentPermissionRequest struct {
	Enabled bool `json:"enabled"`
}
