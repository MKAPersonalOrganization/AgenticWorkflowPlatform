package filesystem

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

// S3VFS is a VFS implementation backed by S3/MinIO for file storage and
// PostgreSQL for directory structure / metadata.  It is fully read-write.
type S3VFS struct {
	client       *minio.Client
	bucket       string
	filesystemID uuid.UUID
	repo         repository
}

// S3Config holds S3/MinIO connection parameters.
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	BucketName      string
	Region          string
}

// NewS3VFSFromClient creates a new S3-backed VFS re-using an existing MinIO
// client (avoids creating a fresh HTTP transport per filesystem).
func NewS3VFSFromClient(client *minio.Client, bucket string, filesystemID uuid.UUID, repo repository) *S3VFS {
	return &S3VFS{
		client:       client,
		bucket:       bucket,
		filesystemID: filesystemID,
		repo:         repo,
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

// s3Key returns the S3 object key for a path inside this filesystem.
func (v *S3VFS) s3Key(p string) string {
	clean := strings.TrimPrefix(path.Clean(p), "/")
	return fmt.Sprintf("fs/%s/%s", v.filesystemID, clean)
}

// splitPath splits "/foo/bar/baz.txt" into parentPath="/foo/bar" and name="baz.txt".
func splitPath(p string) (parentPath, name string, err error) {
	p = path.Clean(p)
	if p == "/" || p == "." || p == "" {
		return "", "", ErrInvalidPath
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	dir := path.Dir(p)
	base := path.Base(p)
	if base == "" || base == "." || base == "/" {
		return "", "", ErrInvalidPath
	}
	return dir, base, nil
}

func normalizePath(p string) string {
	p = path.Clean(p)
	if p == "." || p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// ── VFS implementation ──────────────────────────────────────────────────────

func (v *S3VFS) Mkdir(ctx context.Context, p string) error {
	parentPath, name, err := splitPath(p)
	if err != nil {
		return err
	}

	// Check parent exists (unless creating at root).
	if parentPath != "/" {
		pp, pn, _ := splitPath(parentPath)
		_, err := v.repo.GetEntry(ctx, v.filesystemID, pp, pn)
		if err != nil {
			return fmt.Errorf("parent directory not found: %w", ErrNotFound)
		}
	}

	// Check duplicate.
	_, err = v.repo.GetEntry(ctx, v.filesystemID, parentPath, name)
	if err == nil {
		return ErrAlreadyExists
	}

	entry := FSEntry{
		ID:           uuid.New(),
		FilesystemID: v.filesystemID,
		ParentPath:   parentPath,
		Name:         name,
		EntryType:    EntryTypeDirectory,
		CreatedAt:    now(),
		UpdatedAt:    now(),
	}
	return v.repo.CreateEntry(ctx, entry)
}

func (v *S3VFS) Rmdir(ctx context.Context, p string) error {
	parentPath, name, err := splitPath(p)
	if err != nil {
		return err
	}

	entry, err := v.repo.GetEntry(ctx, v.filesystemID, parentPath, name)
	if err != nil {
		return err
	}
	if entry.EntryType != EntryTypeDirectory {
		return ErrNotDirectory
	}

	// Check directory is empty.
	fullPath := entry.FullPath()
	count, err := v.repo.CountChildren(ctx, v.filesystemID, fullPath)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrNotEmpty
	}

	return v.repo.DeleteEntry(ctx, entry.ID)
}

func (v *S3VFS) ReadDir(ctx context.Context, p string) ([]DirEntry, error) {
	p = normalizePath(p)

	// Verify the directory exists (root always exists).
	if p != "/" {
		pp, pn, err := splitPath(p)
		if err != nil {
			return nil, err
		}
		entry, err := v.repo.GetEntry(ctx, v.filesystemID, pp, pn)
		if err != nil {
			return nil, err
		}
		if entry.EntryType != EntryTypeDirectory {
			return nil, ErrNotDirectory
		}
	}

	entries, err := v.repo.ListEntries(ctx, v.filesystemID, p)
	if err != nil {
		return nil, err
	}

	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = DirEntry{
			Name:        e.Name,
			IsDir:       e.EntryType == EntryTypeDirectory,
			Size:        e.Size,
			ContentType: e.ContentType,
			ModTime:     e.UpdatedAt,
		}
	}
	return result, nil
}

func (v *S3VFS) CreateFile(ctx context.Context, p string, content io.Reader, contentType string, size int64) (*FileStat, error) {
	parentPath, name, err := splitPath(p)
	if err != nil {
		return nil, err
	}

	// Verify parent directory exists (root is implicit).
	if parentPath != "/" {
		pp, pn, _ := splitPath(parentPath)
		parent, err := v.repo.GetEntry(ctx, v.filesystemID, pp, pn)
		if err != nil {
			return nil, fmt.Errorf("parent directory not found: %w", ErrNotFound)
		}
		if parent.EntryType != EntryTypeDirectory {
			return nil, ErrNotDirectory
		}
	}

	key := v.s3Key(p)

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload to S3.
	info, err := v.client.PutObject(ctx, v.bucket, key, content, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("s3vfs: upload: %w", err)
	}

	// Upsert entry in DB.
	existing, dbErr := v.repo.GetEntry(ctx, v.filesystemID, parentPath, name)
	ts := now()
	if dbErr == nil {
		// Update existing file.
		existing.Size = info.Size
		existing.ContentType = contentType
		existing.StorageKey = key
		existing.UpdatedAt = ts
		if err := v.repo.UpdateEntry(ctx, existing); err != nil {
			return nil, err
		}
	} else {
		entry := FSEntry{
			ID:           uuid.New(),
			FilesystemID: v.filesystemID,
			ParentPath:   parentPath,
			Name:         name,
			EntryType:    EntryTypeFile,
			Size:         info.Size,
			ContentType:  contentType,
			StorageKey:   key,
			CreatedAt:    ts,
			UpdatedAt:    ts,
		}
		if err := v.repo.CreateEntry(ctx, entry); err != nil {
			return nil, err
		}
	}

	return &FileStat{
		Name:        name,
		IsDir:       false,
		Size:        info.Size,
		ContentType: contentType,
		ModTime:     ts,
	}, nil
}

func (v *S3VFS) ReadFile(ctx context.Context, p string) (io.ReadCloser, *FileStat, error) {
	parentPath, name, err := splitPath(p)
	if err != nil {
		return nil, nil, err
	}

	entry, err := v.repo.GetEntry(ctx, v.filesystemID, parentPath, name)
	if err != nil {
		return nil, nil, err
	}
	if entry.EntryType != EntryTypeFile {
		return nil, nil, ErrNotFile
	}

	obj, err := v.client.GetObject(ctx, v.bucket, entry.StorageKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("s3vfs: read: %w", err)
	}

	return obj, &FileStat{
		Name:        entry.Name,
		IsDir:       false,
		Size:        entry.Size,
		ContentType: entry.ContentType,
		ModTime:     entry.UpdatedAt,
	}, nil
}

func (v *S3VFS) Remove(ctx context.Context, p string) error {
	parentPath, name, err := splitPath(p)
	if err != nil {
		return err
	}

	entry, err := v.repo.GetEntry(ctx, v.filesystemID, parentPath, name)
	if err != nil {
		return err
	}
	if entry.EntryType != EntryTypeFile {
		return ErrNotFile
	}

	// Remove from S3.
	if err := v.client.RemoveObject(ctx, v.bucket, entry.StorageKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3vfs: remove: %w", err)
	}

	return v.repo.DeleteEntry(ctx, entry.ID)
}

func (v *S3VFS) Stat(ctx context.Context, p string) (*FileStat, error) {
	p = normalizePath(p)
	if p == "/" {
		return &FileStat{Name: "/", IsDir: true, ModTime: time.Now()}, nil
	}

	parentPath, name, err := splitPath(p)
	if err != nil {
		return nil, err
	}

	entry, err := v.repo.GetEntry(ctx, v.filesystemID, parentPath, name)
	if err != nil {
		return nil, err
	}

	return &FileStat{
		Name:        entry.Name,
		IsDir:       entry.EntryType == EntryTypeDirectory,
		Size:        entry.Size,
		ContentType: entry.ContentType,
		ModTime:     entry.UpdatedAt,
	}, nil
}

func (v *S3VFS) Rename(ctx context.Context, oldPath, newPath string) error {
	oldParent, oldName, err := splitPath(oldPath)
	if err != nil {
		return err
	}
	newParent, newName, err := splitPath(newPath)
	if err != nil {
		return err
	}

	entry, err := v.repo.GetEntry(ctx, v.filesystemID, oldParent, oldName)
	if err != nil {
		return err
	}

	// For files, we also need to move the S3 object.
	if entry.EntryType == EntryTypeFile {
		oldKey := entry.StorageKey
		newKey := v.s3Key(newPath)

		// Copy then delete.
		_, err := v.client.CopyObject(ctx, minio.CopyDestOptions{
			Bucket: v.bucket,
			Object: newKey,
		}, minio.CopySrcOptions{
			Bucket: v.bucket,
			Object: oldKey,
		})
		if err != nil {
			return fmt.Errorf("s3vfs: copy for rename: %w", err)
		}

		if err := v.client.RemoveObject(ctx, v.bucket, oldKey, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("s3vfs: remove old key: %w", err)
		}

		entry.StorageKey = newKey
	}

	entry.ParentPath = newParent
	entry.Name = newName
	entry.UpdatedAt = now()
	return v.repo.UpdateEntry(ctx, entry)
}

// compile-time check
var _ VFS = (*S3VFS)(nil)
