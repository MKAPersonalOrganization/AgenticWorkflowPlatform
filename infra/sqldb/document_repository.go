package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"school-with-ai/domain"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// PostgresDocumentRepository implements the DocumentRepository interface using PostgreSQL.
type PostgresDocumentRepository struct {
	db *sqlx.DB
}

// NewPostgresDocumentRepository creates a new PostgresDocumentRepository.
func NewPostgresDocumentRepository(db *sqlx.DB) *PostgresDocumentRepository {
	return &PostgresDocumentRepository{db: db}
}

// Save saves a document to the database.
func (r *PostgresDocumentRepository) Save(ctx context.Context, doc *domain.Document) error {
	query := `
		INSERT INTO documents (id, owner_id, name, parent_path, s3_key, content_type, size, status, is_dir, created_at, updated_at)
		VALUES (:id, :owner_id, :name, :parent_path, :s3_key, :content_type, :size, :status, :is_dir, :created_at, :updated_at)
		ON CONFLICT (id) DO UPDATE SET
			owner_id = :owner_id,
			name = :name,
			parent_path = :parent_path,
			s3_key = :s3_key,
			content_type = :content_type,
			size = :size,
			status = :status,
			is_dir = :is_dir,
			updated_at = :updated_at
	`
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	now := time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	_, err := r.db.NamedExecContext(ctx, query, doc)
	return err
}

// GetByID retrieves a document by its ID.
func (r *PostgresDocumentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Document, error) {
	query := `SELECT * FROM documents WHERE id = $1`
	var doc domain.Document
	err := r.db.GetContext(ctx, &doc, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}
	return &doc, nil
}

// GetByS3Key retrieves a document by its S3 key.
func (r *PostgresDocumentRepository) GetByS3Key(ctx context.Context, s3Key string) (*domain.Document, error) {
	query := `SELECT * FROM documents WHERE s3_key = $1`
	var doc domain.Document
	err := r.db.GetContext(ctx, &doc, query, s3Key)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}
	return &doc, nil
}

// List retrieves a list of documents, optionally filtered by owner and prefix (name).
// Excludes trashed documents.
func (r *PostgresDocumentRepository) List(ctx context.Context, ownerID string, prefix string, limit, offset int) ([]domain.Document, int, error) {
	where := "WHERE deleted_at IS NULL"
	args := []interface{}{}
	argIdx := 1

	if ownerID != "" {
		where += fmt.Sprintf(" AND owner_id = $%d", argIdx)
		args = append(args, ownerID)
		argIdx++
	}

	if prefix != "" {
		where += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, prefix+"%") // Prefix match
		argIdx++
	}

	// Count query
	countQuery := "SELECT COUNT(*) FROM documents " + where
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	// Data query
	query := fmt.Sprintf("SELECT * FROM documents %s ORDER BY is_dir DESC, name ASC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var docs []domain.Document
	if err := r.db.SelectContext(ctx, &docs, query, args...); err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// ListByParentPath retrieves documents that are direct children of the given parent path.
// Excludes trashed documents.
func (r *PostgresDocumentRepository) ListByParentPath(ctx context.Context, ownerID string, parentPath string, limit, offset int) ([]domain.Document, int, error) {
	where := "WHERE deleted_at IS NULL AND parent_path = $1"
	args := []interface{}{parentPath}
	argIdx := 2

	if ownerID != "" {
		where += fmt.Sprintf(" AND owner_id = $%d", argIdx)
		args = append(args, ownerID)
		argIdx++
	}

	// Count query
	countQuery := "SELECT COUNT(*) FROM documents " + where
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	// Data query – folders first, then files alphabetically
	query := fmt.Sprintf("SELECT * FROM documents %s ORDER BY is_dir DESC, name ASC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var docs []domain.Document
	if err := r.db.SelectContext(ctx, &docs, query, args...); err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// Delete deletes a document by ID.
func (r *PostgresDocumentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM documents WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UpdateStatus updates the status of a document.
func (r *PostgresDocumentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.DocumentStatus) error {
	query := `UPDATE documents SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

// Rename updates the name of a document.
func (r *PostgresDocumentRepository) Rename(ctx context.Context, id uuid.UUID, newName string, newS3Key string) error {
	query := `UPDATE documents SET name = $1, s3_key = $2, updated_at = $3 WHERE id = $4`
	_, err := r.db.ExecContext(ctx, query, newName, newS3Key, time.Now(), id)
	return err
}

// Move updates the parent_path, name, and s3_key of a document.
func (r *PostgresDocumentRepository) Move(ctx context.Context, id uuid.UUID, newParentPath, newName, newS3Key string) error {
	query := `UPDATE documents SET parent_path = $1, name = $2, s3_key = $3, updated_at = $4 WHERE id = $5`
	_, err := r.db.ExecContext(ctx, query, newParentPath, newName, newS3Key, time.Now(), id)
	return err
}

// SoftDelete sets deleted_at to mark a document as trashed.
func (r *PostgresDocumentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE documents SET deleted_at = $1, updated_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

// SoftDeleteByPrefix sets deleted_at for all documents matching a prefix (for recursive directory trash).
func (r *PostgresDocumentRepository) SoftDeleteByPrefix(ctx context.Context, prefix string) error {
	query := `UPDATE documents SET deleted_at = $1, updated_at = $1 WHERE name LIKE $2 AND deleted_at IS NULL`
	_, err := r.db.ExecContext(ctx, query, time.Now(), prefix+"%")
	return err
}

// Restore clears deleted_at to restore a document from trash.
func (r *PostgresDocumentRepository) Restore(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE documents SET deleted_at = NULL, updated_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}

// RestoreByPrefix clears deleted_at for all documents matching a prefix.
func (r *PostgresDocumentRepository) RestoreByPrefix(ctx context.Context, prefix string) error {
	query := `UPDATE documents SET deleted_at = NULL, updated_at = $1 WHERE name LIKE $2 AND deleted_at IS NOT NULL`
	_, err := r.db.ExecContext(ctx, query, time.Now(), prefix+"%")
	return err
}

// ListTrashed retrieves trashed documents.
func (r *PostgresDocumentRepository) ListTrashed(ctx context.Context, ownerID string, limit, offset int) ([]domain.Document, int, error) {
	where := "WHERE deleted_at IS NOT NULL"
	args := []interface{}{}
	argIdx := 1

	if ownerID != "" {
		where += fmt.Sprintf(" AND owner_id = $%d", argIdx)
		args = append(args, ownerID)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM documents " + where
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("SELECT * FROM documents %s ORDER BY deleted_at DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var docs []domain.Document
	if err := r.db.SelectContext(ctx, &docs, query, args...); err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// EmptyTrash permanently deletes all trashed documents for an owner.
// Returns the list of trashed documents before deletion (for S3 cleanup).
func (r *PostgresDocumentRepository) EmptyTrash(ctx context.Context, ownerID string) ([]domain.Document, error) {
	where := "WHERE deleted_at IS NOT NULL"
	args := []interface{}{}
	argIdx := 1

	if ownerID != "" {
		where += fmt.Sprintf(" AND owner_id = $%d", argIdx)
		args = append(args, ownerID)
		argIdx++
	}

	// Get list first for S3 cleanup
	selectQuery := "SELECT * FROM documents " + where
	var docs []domain.Document
	if err := r.db.SelectContext(ctx, &docs, selectQuery, args...); err != nil {
		return nil, err
	}

	// Delete them
	deleteQuery := "DELETE FROM documents " + where
	if _, err := r.db.ExecContext(ctx, deleteQuery, args...); err != nil {
		return nil, err
	}

	return docs, nil
}

// SearchByName searches documents by name using ILIKE.
// Excludes trashed documents.
func (r *PostgresDocumentRepository) SearchByName(ctx context.Context, ownerID, query string, limit, offset int) ([]domain.Document, int, error) {
	where := "WHERE deleted_at IS NULL AND name ILIKE $1"
	args := []interface{}{"%" + query + "%"}
	argIdx := 2

	if ownerID != "" {
		where += fmt.Sprintf(" AND owner_id = $%d", argIdx)
		args = append(args, ownerID)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM documents " + where
	var total int
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	selectQuery := fmt.Sprintf("SELECT * FROM documents %s ORDER BY is_dir DESC, name ASC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
	args = append(args, limit, offset)

	var docs []domain.Document
	if err := r.db.SelectContext(ctx, &docs, selectQuery, args...); err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// DeleteByPrefix permanently deletes all documents matching a prefix.
func (r *PostgresDocumentRepository) DeleteByPrefix(ctx context.Context, prefix string) error {
	query := `DELETE FROM documents WHERE name LIKE $1`
	_, err := r.db.ExecContext(ctx, query, prefix+"%")
	return err
}
