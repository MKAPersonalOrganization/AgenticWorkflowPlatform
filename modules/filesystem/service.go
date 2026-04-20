package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"aicompass/modules/filesystem/githubauth"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// service encapsulates the business logic for filesystems.
type service struct {
	repo     repository
	s3Client *minio.Client
	s3Bucket string
	s3Config S3Config
	ghStore  *githubauth.Store
}

func newService(repo repository, cfg S3Config, ghStore *githubauth.Store) (*service, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("filesystem: s3 connect: %w", err)
	}

	return &service{
		repo:     repo,
		s3Client: client,
		s3Bucket: cfg.BucketName,
		s3Config: cfg,
		ghStore:  ghStore,
	}, nil
}

// ── Filesystem CRUD ─────────────────────────────────────────────────────────

func (s *service) createFilesystem(ctx context.Context, userID string, req CreateFilesystemRequest) (FileSystem, error) {
	if req.Name == "" {
		return FileSystem{}, fmt.Errorf("name is required")
	}
	if req.Type == "" {
		req.Type = FSTypeLocal
	}
	if req.Config == nil {
		req.Config = json.RawMessage(`{}`)
	}

	// Validate GitHub config if type is github.
	if req.Type == FSTypeGitHub {
		var ghCfg GitHubConfig
		if err := json.Unmarshal(req.Config, &ghCfg); err != nil {
			return FileSystem{}, fmt.Errorf("invalid github config: %w", err)
		}
		if ghCfg.Owner == "" || ghCfg.Repo == "" {
			return FileSystem{}, fmt.Errorf("github config requires owner and repo")
		}
		if ghCfg.Branch == "" {
			ghCfg.Branch = "main"
		}
		// Never persist the access token inside the filesystem config —
		// it will be resolved at runtime from the github_connections table.
		ghCfg.Token = ""
		req.Config, _ = json.Marshal(ghCfg)
	}

	ts := time.Now().UTC()
	fs := FileSystem{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		Config:      req.Config,
		CreatedAt:   ts,
		UpdatedAt:   ts,
	}

	if err := s.repo.CreateFilesystem(ctx, fs); err != nil {
		return FileSystem{}, err
	}
	return fs, nil
}

func (s *service) listFilesystems(ctx context.Context, userID string) ([]FileSystem, error) {
	return s.repo.ListFilesystems(ctx, userID)
}

func (s *service) getFilesystem(ctx context.Context, id uuid.UUID, userID string) (FileSystem, error) {
	fs, err := s.repo.GetFilesystem(ctx, id)
	if err != nil {
		return fs, err
	}
	if fs.UserID != userID {
		return FileSystem{}, ErrNotFound
	}
	return fs, nil
}

func (s *service) updateFilesystem(ctx context.Context, id uuid.UUID, userID string, req UpdateFilesystemRequest) (FileSystem, error) {
	fs, err := s.repo.GetFilesystem(ctx, id)
	if err != nil {
		return fs, err
	}
	if fs.UserID != userID {
		return FileSystem{}, ErrNotFound
	}

	if req.Name != nil {
		fs.Name = *req.Name
	}
	if req.Description != nil {
		fs.Description = *req.Description
	}
	fs.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateFilesystem(ctx, fs); err != nil {
		return fs, err
	}
	return fs, nil
}

func (s *service) deleteFilesystem(ctx context.Context, id uuid.UUID, userID string) error {
	return s.repo.DeleteFilesystem(ctx, id, userID)
}

// ── VFS Access ──────────────────────────────────────────────────────────────

// vfsFor returns the appropriate VFS backend for a filesystem.
func (s *service) vfsFor(ctx context.Context, fs FileSystem) (VFS, error) {
	switch fs.Type {
	case FSTypeLocal:
		return NewS3VFSFromClient(s.s3Client, s.s3Bucket, fs.ID, s.repo), nil
	case FSTypeGitHub:
		var cfg GitHubConfig
		if err := json.Unmarshal(fs.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid github config: %w", err)
		}
		// Resolve OAuth token from github_connections table if the config
		// does not already contain a PAT and a store is available.
		if cfg.Token == "" && s.ghStore != nil {
			token, err := s.ghStore.GetToken(ctx, fs.UserID)
			if err == nil && token != "" {
				cfg.Token = token
			}
		}
		return NewGitHubVFS(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported filesystem type: %s", fs.Type)
	}
}

// ── File Operations ─────────────────────────────────────────────────────────

func (s *service) listDir(ctx context.Context, fsID uuid.UUID, userID UserID, dirPath string) ([]DirEntry, error) {
	fs, err := s.getFilesystem(ctx, fsID, string(userID))
	if err != nil {
		return nil, err
	}
	vfs, err := s.vfsFor(ctx, fs)
	if err != nil {
		return nil, err
	}
	return vfs.ReadDir(ctx, dirPath)
}

func (s *service) mkdir(ctx context.Context, fsID uuid.UUID, userID UserID, dirPath string) error {
	fs, err := s.getFilesystem(ctx, fsID, string(userID))
	if err != nil {
		return err
	}
	vfs, err := s.vfsFor(ctx, fs)
	if err != nil {
		return err
	}
	return vfs.Mkdir(ctx, dirPath)
}

func (s *service) uploadFile(ctx context.Context, fsID uuid.UUID, userID UserID, filePath string, content io.Reader, contentType string, size int64) (*FileStat, error) {
	fs, err := s.getFilesystem(ctx, fsID, string(userID))
	if err != nil {
		return nil, err
	}
	vfs, err := s.vfsFor(ctx, fs)
	if err != nil {
		return nil, err
	}
	return vfs.CreateFile(ctx, filePath, content, contentType, size)
}

func (s *service) readFile(ctx context.Context, fsID uuid.UUID, userID UserID, filePath string) (io.ReadCloser, *FileStat, error) {
	fs, err := s.getFilesystem(ctx, fsID, string(userID))
	if err != nil {
		return nil, nil, err
	}
	vfs, err := s.vfsFor(ctx, fs)
	if err != nil {
		return nil, nil, err
	}
	return vfs.ReadFile(ctx, filePath)
}

func (s *service) removeEntry(ctx context.Context, fsID uuid.UUID, userID UserID, entryPath string) error {
	fs, err := s.getFilesystem(ctx, fsID, string(userID))
	if err != nil {
		return err
	}
	vfs, err := s.vfsFor(ctx, fs)
	if err != nil {
		return err
	}

	stat, err := vfs.Stat(ctx, entryPath)
	if err != nil {
		return err
	}

	if stat.IsDir {
		return vfs.Rmdir(ctx, entryPath)
	}
	return vfs.Remove(ctx, entryPath)
}

func (s *service) renameEntry(ctx context.Context, fsID uuid.UUID, userID UserID, req RenameRequest) error {
	fs, err := s.getFilesystem(ctx, fsID, string(userID))
	if err != nil {
		return err
	}
	vfs, err := s.vfsFor(ctx, fs)
	if err != nil {
		return err
	}
	return vfs.Rename(ctx, req.OldPath, req.NewPath)
}

// ── Permissions ─────────────────────────────────────────────────────────────

func (s *service) grantPermission(ctx context.Context, fsID uuid.UUID, userID UserID, agentID AgentID) error {
	// Verify the user owns this filesystem.
	if _, err := s.getFilesystem(ctx, fsID, string(userID)); err != nil {
		return err
	}

	return s.repo.GrantPermission(ctx, FSPermission{
		ID:           uuid.New(),
		FilesystemID: fsID,
		UserID:       string(userID),
		AgentID:      string(agentID),
		GrantedAt:    time.Now().UTC(),
	})
}

func (s *service) revokePermission(ctx context.Context, fsID uuid.UUID, userID UserID, agentID AgentID) error {
	if _, err := s.getFilesystem(ctx, fsID, string(userID)); err != nil {
		return err
	}
	return s.repo.RevokePermission(ctx, fsID, userID, agentID)
}

func (s *service) listPermissions(ctx context.Context, fsID uuid.UUID, userID string) ([]FSPermission, error) {
	if _, err := s.getFilesystem(ctx, fsID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListPermissions(ctx, fsID, userID)
}

// ── Agent Tool Interface ────────────────────────────────────────────────────

func (s *service) listPermittedFilesystems(ctx context.Context, userID UserID, agentID AgentID) ([]FileSystem, error) {
	return s.repo.ListPermittedFilesystems(ctx, userID, agentID)
}

func (s *service) hasAgentPermission(ctx context.Context, fsID uuid.UUID, userID UserID, agentID AgentID) (bool, error) {
	return s.repo.HasPermission(ctx, fsID, userID, agentID)
}

func (s *service) listAgentFilesystems(ctx context.Context, userID UserID, agentID AgentID) ([]AgentFilesystem, error) {
	// 1. Get all filesystems owned by the user
	allFS, err := s.repo.ListFilesystems(ctx, string(userID))
	if err != nil {
		return nil, err
	}

	// 2. Get filesystems the agent already has access to
	permittedFS, err := s.repo.ListPermittedFilesystems(ctx, userID, agentID)
	if err != nil {
		return nil, err
	}

	// 3. Create a map for quick lookup
	permittedMap := make(map[uuid.UUID]bool)
	for _, fs := range permittedFS {
		permittedMap[fs.ID] = true
	}

	// 4. Merge results
	out := make([]AgentFilesystem, 0, len(allFS))
	for _, fs := range allFS {
		out = append(out, AgentFilesystem{
			FileSystem: fs,
			Enabled:    permittedMap[fs.ID],
		})
	}

	return out, nil
}
