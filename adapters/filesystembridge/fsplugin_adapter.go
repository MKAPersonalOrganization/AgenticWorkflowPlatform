package filesystembridge

import (
	"aicompass/modules/filesystem"
	"aicompass/plugins/fsplugin"
	"context"
	"io"

	"github.com/google/uuid"
)

// FSPluginAdapter bridges modules/filesystem.AgentFS to fsplugin.AgentFilesystem.
type FSPluginAdapter struct {
	source filesystem.AgentFS
}

// NewFSPluginAdapter creates a bridge from the filesystem module to fsplugin.
func NewFSPluginAdapter(source filesystem.AgentFS) *FSPluginAdapter {
	return &FSPluginAdapter{source: source}
}

func (a *FSPluginAdapter) ListPermittedFilesystems(ctx context.Context, userID, agentID string) ([]fsplugin.FileSystem, error) {
	fss, err := a.source.ListPermittedFilesystems(ctx, filesystem.UserID(userID), filesystem.AgentID(agentID))
	if err != nil {
		return nil, err
	}
	out := make([]fsplugin.FileSystem, len(fss))
	for i, f := range fss {
		out[i] = fsplugin.FileSystem{
			ID:          f.ID,
			UserID:      f.UserID,
			Name:        f.Name,
			Description: f.Description,
			Type:        string(f.Type),
		}
	}
	return out, nil
}

func (a *FSPluginAdapter) HasAgentPermission(ctx context.Context, fsID uuid.UUID, userID, agentID string) (bool, error) {
	return a.source.HasAgentPermission(ctx, fsID, filesystem.UserID(userID), filesystem.AgentID(agentID))
}

func (a *FSPluginAdapter) ListDir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) ([]fsplugin.DirEntry, error) {
	entries, err := a.source.ListDir(ctx, fsID, filesystem.UserID(userID), dirPath)
	if err != nil {
		return nil, err
	}
	out := make([]fsplugin.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = fsplugin.DirEntry{
			Name:        e.Name,
			IsDir:       e.IsDir,
			Size:        e.Size,
			ContentType: e.ContentType,
			ModTime:     e.ModTime,
		}
	}
	return out, nil
}

func (a *FSPluginAdapter) Mkdir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) error {
	return a.source.Mkdir(ctx, fsID, filesystem.UserID(userID), dirPath)
}

func (a *FSPluginAdapter) ReadFile(ctx context.Context, fsID uuid.UUID, userID, filePath string) (io.ReadCloser, *fsplugin.FileStat, error) {
	rc, stat, err := a.source.ReadFile(ctx, fsID, filesystem.UserID(userID), filePath)
	if err != nil {
		return nil, nil, err
	}
	return rc, mapFileStat(stat), nil
}

func (a *FSPluginAdapter) WriteFile(ctx context.Context, fsID uuid.UUID, userID, filePath string, content io.Reader, contentType string, size int64) (*fsplugin.FileStat, error) {
	stat, err := a.source.WriteFile(ctx, fsID, filesystem.UserID(userID), filePath, content, contentType, size)
	if err != nil {
		return nil, err
	}
	return mapFileStat(stat), nil
}

func (a *FSPluginAdapter) Remove(ctx context.Context, fsID uuid.UUID, userID, entryPath string) error {
	return a.source.Remove(ctx, fsID, filesystem.UserID(userID), entryPath)
}

func (a *FSPluginAdapter) Stat(ctx context.Context, fsID uuid.UUID, userID, filePath string) (*fsplugin.FileStat, error) {
	stat, err := a.source.Stat(ctx, fsID, filesystem.UserID(userID), filePath)
	if err != nil {
		return nil, err
	}
	return mapFileStat(stat), nil
}

func (a *FSPluginAdapter) Rename(ctx context.Context, fsID uuid.UUID, userID, oldPath, newPath string) error {
	return a.source.Rename(ctx, fsID, filesystem.UserID(userID), oldPath, newPath)
}

func mapFileStat(s *filesystem.FileStat) *fsplugin.FileStat {
	if s == nil {
		return nil
	}
	return &fsplugin.FileStat{
		Name:        s.Name,
		IsDir:       s.IsDir,
		Size:        s.Size,
		ContentType: s.ContentType,
		ModTime:     s.ModTime,
	}
}
