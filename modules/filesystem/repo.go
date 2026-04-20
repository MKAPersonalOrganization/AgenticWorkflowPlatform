package filesystem

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ─── Repository Interface ───────────────────────────────────────────────────

type repository interface {
	// ── Filesystem CRUD ─────────────────────────────────────────────────
	CreateFilesystem(ctx context.Context, fs FileSystem) error
	ListFilesystems(ctx context.Context, userID string) ([]FileSystem, error)
	GetFilesystem(ctx context.Context, id uuid.UUID) (FileSystem, error)
	UpdateFilesystem(ctx context.Context, fs FileSystem) error
	DeleteFilesystem(ctx context.Context, id uuid.UUID, userID string) error

	// ── Entry CRUD ──────────────────────────────────────────────────────
	CreateEntry(ctx context.Context, entry FSEntry) error
	ListEntries(ctx context.Context, filesystemID uuid.UUID, parentPath string) ([]FSEntry, error)
	GetEntry(ctx context.Context, filesystemID uuid.UUID, parentPath, name string) (FSEntry, error)
	GetEntryByID(ctx context.Context, id uuid.UUID) (FSEntry, error)
	UpdateEntry(ctx context.Context, entry FSEntry) error
	DeleteEntry(ctx context.Context, id uuid.UUID) error
	CountChildren(ctx context.Context, filesystemID uuid.UUID, parentPath string) (int, error)

	// ── Permissions ─────────────────────────────────────────────────────
	GrantPermission(ctx context.Context, perm FSPermission) error
	RevokePermission(ctx context.Context, filesystemID uuid.UUID, userID UserID, agentID AgentID) error
	ListPermissions(ctx context.Context, filesystemID uuid.UUID, userID string) ([]FSPermission, error)
	HasPermission(ctx context.Context, filesystemID uuid.UUID, userID UserID, agentID AgentID) (bool, error)
	// ListPermittedFilesystems returns filesystems that an agent has been granted access to.
	ListPermittedFilesystems(ctx context.Context, userID UserID, agentID AgentID) ([]FileSystem, error)
}

// ─── PostgreSQL Implementation ──────────────────────────────────────────────

type postgresRepo struct {
	db *sqlx.DB
}

func newPostgresRepo(db *sqlx.DB) *postgresRepo {
	return &postgresRepo{db: db}
}

// ── Filesystem CRUD ─────────────────────────────────────────────────────────

func (r *postgresRepo) CreateFilesystem(ctx context.Context, fs FileSystem) error {
	const q = `INSERT INTO filesystems (filesystem_id, user_id, name, description, fs_type, config, created_at, updated_at)
		VALUES (:filesystem_id, :user_id, :name, :description, :fs_type, :config, :created_at, :updated_at)`
	_, err := r.db.NamedExecContext(ctx, q, fs)
	return err
}

func (r *postgresRepo) ListFilesystems(ctx context.Context, userID string) ([]FileSystem, error) {
	var out []FileSystem
	const q = `SELECT * FROM filesystems WHERE user_id = $1 ORDER BY created_at DESC LIMIT 200`
	err := r.db.SelectContext(ctx, &out, q, userID)
	return out, err
}

func (r *postgresRepo) GetFilesystem(ctx context.Context, id uuid.UUID) (FileSystem, error) {
	var fs FileSystem
	const q = `SELECT * FROM filesystems WHERE filesystem_id = $1`
	err := r.db.GetContext(ctx, &fs, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return fs, ErrNotFound
	}
	return fs, err
}

func (r *postgresRepo) UpdateFilesystem(ctx context.Context, fs FileSystem) error {
	const q = `UPDATE filesystems SET name = :name, description = :description, updated_at = :updated_at WHERE filesystem_id = :filesystem_id AND user_id = :user_id`
	res, err := r.db.NamedExecContext(ctx, q, fs)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepo) DeleteFilesystem(ctx context.Context, id uuid.UUID, userID string) error {
	const q = `DELETE FROM filesystems WHERE filesystem_id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Entry CRUD ──────────────────────────────────────────────────────────────

func (r *postgresRepo) CreateEntry(ctx context.Context, entry FSEntry) error {
	const q = `INSERT INTO filesystem_entries (entry_id, filesystem_id, parent_path, name, entry_type, size, content_type, storage_key, created_at, updated_at)
		VALUES (:entry_id, :filesystem_id, :parent_path, :name, :entry_type, :size, :content_type, :storage_key, :created_at, :updated_at)`
	_, err := r.db.NamedExecContext(ctx, q, entry)
	return err
}

func (r *postgresRepo) ListEntries(ctx context.Context, filesystemID uuid.UUID, parentPath string) ([]FSEntry, error) {
	var out []FSEntry
	const q = `SELECT * FROM filesystem_entries WHERE filesystem_id = $1 AND parent_path = $2 ORDER BY entry_type ASC, name ASC`
	err := r.db.SelectContext(ctx, &out, q, filesystemID, parentPath)
	return out, err
}

func (r *postgresRepo) GetEntry(ctx context.Context, filesystemID uuid.UUID, parentPath, name string) (FSEntry, error) {
	var entry FSEntry
	const q = `SELECT * FROM filesystem_entries WHERE filesystem_id = $1 AND parent_path = $2 AND name = $3`
	err := r.db.GetContext(ctx, &entry, q, filesystemID, parentPath, name)
	if errors.Is(err, sql.ErrNoRows) {
		return entry, ErrNotFound
	}
	return entry, err
}

func (r *postgresRepo) GetEntryByID(ctx context.Context, id uuid.UUID) (FSEntry, error) {
	var entry FSEntry
	const q = `SELECT * FROM filesystem_entries WHERE entry_id = $1`
	err := r.db.GetContext(ctx, &entry, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return entry, ErrNotFound
	}
	return entry, err
}

func (r *postgresRepo) UpdateEntry(ctx context.Context, entry FSEntry) error {
	const q = `UPDATE filesystem_entries SET parent_path = :parent_path, name = :name, size = :size, content_type = :content_type, storage_key = :storage_key, updated_at = :updated_at
		WHERE entry_id = :entry_id`
	_, err := r.db.NamedExecContext(ctx, q, entry)
	return err
}

func (r *postgresRepo) DeleteEntry(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM filesystem_entries WHERE entry_id = $1`
	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepo) CountChildren(ctx context.Context, filesystemID uuid.UUID, parentPath string) (int, error) {
	var count int
	const q = `SELECT COUNT(*) FROM filesystem_entries WHERE filesystem_id = $1 AND parent_path = $2`
	err := r.db.GetContext(ctx, &count, q, filesystemID, parentPath)
	return count, err
}

// ── Permissions ─────────────────────────────────────────────────────────────

func (r *postgresRepo) GrantPermission(ctx context.Context, perm FSPermission) error {
	const q = `INSERT INTO filesystem_permissions (permission_id, filesystem_id, user_id, agent_id, granted_at)
		VALUES (:permission_id, :filesystem_id, :user_id, :agent_id, :granted_at)
		ON CONFLICT (filesystem_id, user_id, agent_id) DO NOTHING`
	_, err := r.db.NamedExecContext(ctx, q, perm)
	return err
}

func (r *postgresRepo) RevokePermission(ctx context.Context, filesystemID uuid.UUID, userID UserID, agentID AgentID) error {
	const q = `DELETE FROM filesystem_permissions WHERE filesystem_id = $1 AND user_id = $2 AND agent_id = $3`
	res, err := r.db.ExecContext(ctx, q, filesystemID, userID, agentID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("permission not found")
	}
	return nil
}

func (r *postgresRepo) ListPermissions(ctx context.Context, filesystemID uuid.UUID, userID string) ([]FSPermission, error) {
	var out []FSPermission
	const q = `SELECT * FROM filesystem_permissions WHERE filesystem_id = $1 AND user_id = $2 ORDER BY granted_at DESC`
	err := r.db.SelectContext(ctx, &out, q, filesystemID, userID)
	return out, err
}

func (r *postgresRepo) HasPermission(ctx context.Context, filesystemID uuid.UUID, userID UserID, agentID AgentID) (bool, error) {
	var count int
	const q = `SELECT COUNT(*) FROM filesystem_permissions WHERE filesystem_id = $1 AND user_id = $2 AND agent_id = $3`
	err := r.db.GetContext(ctx, &count, q, filesystemID, userID, agentID)
	return count > 0, err
}

func (r *postgresRepo) ListPermittedFilesystems(ctx context.Context, userID UserID, agentID AgentID) ([]FileSystem, error) {
	var out []FileSystem
	const q = `
		SELECT f.filesystem_id, f.user_id, f.name, f.description, f.fs_type, f.config, f.created_at, f.updated_at
		FROM filesystems f
		INNER JOIN filesystem_permissions fp ON f.filesystem_id = fp.filesystem_id AND f.user_id = fp.user_id
		WHERE fp.user_id = $1 AND fp.agent_id = $2
		ORDER BY f.name ASC`
	err := r.db.SelectContext(ctx, &out, q, userID, agentID)
	return out, err
}

// compile-time check
var _ repository = (*postgresRepo)(nil)

// ── Helpers ─────────────────────────────────────────────────────────────────

func now() time.Time { return time.Now().UTC() }
