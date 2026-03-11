package custom

import (
	"context"
	"io"

	"school-with-ai/modules/filesystem"
	"school-with-ai/plugins/fsplugin"

	"github.com/google/uuid"
)

// AgentFSAdapter bridges modules/filesystem.AgentFS → fsplugin.AgentFilesystem.
// It lives in the composition-root adapter layer and is the only place that
// imports both packages simultaneously.
type AgentFSAdapter struct {
	Source filesystem.AgentFS
}

func (a *AgentFSAdapter) ListPermittedFilesystems(ctx context.Context, userID, agentID string) ([]fsplugin.FileSystem, error) {
	fss, err := a.Source.ListPermittedFilesystems(ctx, userID, agentID)
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

func (a *AgentFSAdapter) HasAgentPermission(ctx context.Context, fsID uuid.UUID, userID, agentID string) (bool, error) {
	return a.Source.HasAgentPermission(ctx, fsID, userID, agentID)
}

func (a *AgentFSAdapter) ListDir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) ([]fsplugin.DirEntry, error) {
	entries, err := a.Source.ListDir(ctx, fsID, userID, dirPath)
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

func (a *AgentFSAdapter) Mkdir(ctx context.Context, fsID uuid.UUID, userID, dirPath string) error {
	return a.Source.Mkdir(ctx, fsID, userID, dirPath)
}

func (a *AgentFSAdapter) ReadFile(ctx context.Context, fsID uuid.UUID, userID, filePath string) (io.ReadCloser, *fsplugin.FileStat, error) {
	rc, stat, err := a.Source.ReadFile(ctx, fsID, userID, filePath)
	if err != nil {
		return nil, nil, err
	}
	return rc, mapFileStat(stat), nil
}

func (a *AgentFSAdapter) WriteFile(ctx context.Context, fsID uuid.UUID, userID, filePath string, content io.Reader, contentType string, size int64) (*fsplugin.FileStat, error) {
	stat, err := a.Source.WriteFile(ctx, fsID, userID, filePath, content, contentType, size)
	if err != nil {
		return nil, err
	}
	return mapFileStat(stat), nil
}

func (a *AgentFSAdapter) Remove(ctx context.Context, fsID uuid.UUID, userID, entryPath string) error {
	return a.Source.Remove(ctx, fsID, userID, entryPath)
}

func (a *AgentFSAdapter) Stat(ctx context.Context, fsID uuid.UUID, userID, filePath string) (*fsplugin.FileStat, error) {
	stat, err := a.Source.Stat(ctx, fsID, userID, filePath)
	if err != nil {
		return nil, err
	}
	return mapFileStat(stat), nil
}

func (a *AgentFSAdapter) Rename(ctx context.Context, fsID uuid.UUID, userID, oldPath, newPath string) error {
	return a.Source.Rename(ctx, fsID, userID, oldPath, newPath)
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
