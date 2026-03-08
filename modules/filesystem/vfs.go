package filesystem

import (
	"context"
	"errors"
	"io"
	"time"
)

// ─── VFS Abstraction ────────────────────────────────────────────────────────
// Modelled after the Linux Virtual File System (VFS) layer.
// Each backend (S3, GitHub, …) implements this interface so that the
// service layer can treat every filesystem uniformly.

var (
	ErrNotFound      = errors.New("filesystem: entry not found")
	ErrAlreadyExists = errors.New("filesystem: entry already exists")
	ErrNotDirectory  = errors.New("filesystem: not a directory")
	ErrNotFile       = errors.New("filesystem: not a file")
	ErrNotEmpty      = errors.New("filesystem: directory not empty")
	ErrReadOnly      = errors.New("filesystem: read-only filesystem")
	ErrInvalidPath   = errors.New("filesystem: invalid path")
)

// DirEntry represents a single entry returned by ReadDir.
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

// VFS is the virtual-filesystem interface.
//
// Implementations are required to normalise paths (always start with "/",
// no trailing slash except for root).  The caller is expected to pre-validate
// ownership/permissions — the VFS layer itself is permission-unaware.
type VFS interface {
	// ── Directory operations ────────────────────────────────────────────

	// Mkdir creates a directory at path. Intermediate directories are NOT
	// created automatically (use MkdirAll for that).
	Mkdir(ctx context.Context, path string) error

	// Rmdir removes an *empty* directory.
	Rmdir(ctx context.Context, path string) error

	// ReadDir returns the direct children of the directory at path.
	ReadDir(ctx context.Context, path string) ([]DirEntry, error)

	// ── File operations ─────────────────────────────────────────────────

	// CreateFile writes content to path. If the file already exists it is
	// overwritten.
	CreateFile(ctx context.Context, path string, content io.Reader, contentType string, size int64) (*FileStat, error)

	// ReadFile returns a ReadCloser for the file's content. The caller
	// MUST close the returned reader.
	ReadFile(ctx context.Context, path string) (io.ReadCloser, *FileStat, error)

	// Remove deletes a single file.
	Remove(ctx context.Context, path string) error

	// ── Metadata ────────────────────────────────────────────────────────

	// Stat returns metadata for the entry at path.
	Stat(ctx context.Context, path string) (*FileStat, error)

	// ── Rename / Move ───────────────────────────────────────────────────

	// Rename moves/renames from oldPath to newPath.
	Rename(ctx context.Context, oldPath, newPath string) error
}
