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
		INSERT INTO documents (id, owner_id, name, s3_key, content_type, size, status, is_dir, created_at, updated_at)
		VALUES (:id, :owner_id, :name, :s3_key, :content_type, :size, :status, :is_dir, :created_at, :updated_at)
		ON CONFLICT (id) DO UPDATE SET
			owner_id = :owner_id,
			name = :name,
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
func (r *PostgresDocumentRepository) List(ctx context.Context, ownerID string, prefix string, limit, offset int) ([]domain.Document, int, error) {
	// Base query
	where := "WHERE 1=1"
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
	query := fmt.Sprintf("SELECT * FROM documents %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, argIdx, argIdx+1)
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
