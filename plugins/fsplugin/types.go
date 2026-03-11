package fsplugin

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// ─── Local DTOs ─────────────────────────────────────────────────────────────
// These mirror the structs defined in modules/filesystem but are owned by the
// plugin so that this package carries no import of modules/.

// FileSystem describes a user-owned virtual filesystem.
type FileSystem struct {
	ID          uuid.UUID
	UserID      string
	Name        string
	Description string
	Type        string // "local", "github", …
}

// DirEntry represents a single entry returned by a directory listing.
type DirEntry struct {
	Name        string    `json:"name"`
	IsDir       bool      `json:"is_dir"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type,omitempty"`
	ModTime     time.Time `json:"mod_time"`
}

// FileStat carries metadata about a file or directory.
type FileStat struct {
	Name        string    `json:"name"`
	IsDir       bool      `json:"is_dir"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type,omitempty"`
	ModTime     time.Time `json:"mod_time"`
}

// ─── Local interface ────────────────────────────────────────────────────────
// AgentFilesystem is the contract the fsplugin tools depend on.
// modules/filesystem.AgentFS does NOT satisfy this directly (different return
// types), so an adapter in adapters/custom bridges the two.

// AgentFilesystem exposes permission-checked filesystem operations.
type AgentFilesystem interface {
	ListPermittedFilesystems(ctx context.Context, userID, agentID string) ([]FileSystem, error)
	HasAgentPermission(ctx context.Context, fsID uuid.UUID, userID, agentID string) (bool, error)
	ListDir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) ([]DirEntry, error)
	Mkdir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) error
	ReadFile(ctx context.Context, fsID uuid.UUID, userID, filePath string) (io.ReadCloser, *FileStat, error)
	WriteFile(ctx context.Context, fsID uuid.UUID, userID, filePath string, content io.Reader, contentType string, size int64) (*FileStat, error)
	Remove(ctx context.Context, fsID uuid.UUID, userID, entryPath string) error
	Stat(ctx context.Context, fsID uuid.UUID, userID, filePath string) (*FileStat, error)
	Rename(ctx context.Context, fsID uuid.UUID, userID, oldPath, newPath string) error
}
