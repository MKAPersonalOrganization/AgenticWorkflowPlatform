package filesystem

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// AgentFS is the interface exposed to agent-facing tools (plugins).
// It wraps the internal service methods with permission-checked access.
type AgentFS interface {
	// ListPermittedFilesystems returns filesystems an agent has been granted access to.
	ListPermittedFilesystems(ctx context.Context, userID, agentID string) ([]FileSystem, error)

	// HasAgentPermission checks if an agent has permission to access a filesystem.
	HasAgentPermission(ctx context.Context, fsID uuid.UUID, userID, agentID string) (bool, error)

	// ListDir returns directory entries.
	ListDir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) ([]DirEntry, error)

	// Mkdir creates a directory.
	Mkdir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) error

	// ReadFile returns the file content and metadata.
	ReadFile(ctx context.Context, fsID uuid.UUID, userID, filePath string) (io.ReadCloser, *FileStat, error)

	// WriteFile creates or overwrites a file.
	WriteFile(ctx context.Context, fsID uuid.UUID, userID, filePath string, content io.Reader, contentType string, size int64) (*FileStat, error)

	// Remove deletes a file or empty directory.
	Remove(ctx context.Context, fsID uuid.UUID, userID, entryPath string) error

	// Stat returns metadata for a path.
	Stat(ctx context.Context, fsID uuid.UUID, userID, filePath string) (*FileStat, error)

	// Rename moves/renames an entry.
	Rename(ctx context.Context, fsID uuid.UUID, userID, oldPath, newPath string) error
}

// agentFSAdapter wraps the internal service to implement AgentFS.
type agentFSAdapter struct {
	svc *service
}

func (a *agentFSAdapter) ListPermittedFilesystems(ctx context.Context, userID, agentID string) ([]FileSystem, error) {
	return a.svc.listPermittedFilesystems(ctx, userID, agentID)
}

func (a *agentFSAdapter) HasAgentPermission(ctx context.Context, fsID uuid.UUID, userID, agentID string) (bool, error) {
	return a.svc.hasAgentPermission(ctx, fsID, userID, agentID)
}

func (a *agentFSAdapter) ListDir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) ([]DirEntry, error) {
	return a.svc.listDir(ctx, fsID, userID, dirPath)
}

func (a *agentFSAdapter) Mkdir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) error {
	return a.svc.mkdir(ctx, fsID, userID, dirPath)
}

func (a *agentFSAdapter) ReadFile(ctx context.Context, fsID uuid.UUID, userID, filePath string) (io.ReadCloser, *FileStat, error) {
	return a.svc.readFile(ctx, fsID, userID, filePath)
}

func (a *agentFSAdapter) WriteFile(ctx context.Context, fsID uuid.UUID, userID, filePath string, content io.Reader, contentType string, size int64) (*FileStat, error) {
	return a.svc.uploadFile(ctx, fsID, userID, filePath, content, contentType, size)
}

func (a *agentFSAdapter) Remove(ctx context.Context, fsID uuid.UUID, userID, entryPath string) error {
	return a.svc.removeEntry(ctx, fsID, userID, entryPath)
}

func (a *agentFSAdapter) Stat(ctx context.Context, fsID uuid.UUID, userID, filePath string) (*FileStat, error) {
	fs, err := a.svc.getFilesystem(ctx, fsID, userID)
	if err != nil {
		return nil, err
	}
	vfs, err := a.svc.vfsFor(ctx, fs)
	if err != nil {
		return nil, err
	}
	return vfs.Stat(ctx, filePath)
}

func (a *agentFSAdapter) Rename(ctx context.Context, fsID uuid.UUID, userID, oldPath, newPath string) error {
	return a.svc.renameEntry(ctx, fsID, userID, RenameRequest{OldPath: oldPath, NewPath: newPath})
}
