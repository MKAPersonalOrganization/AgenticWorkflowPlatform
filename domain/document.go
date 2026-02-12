package domain

import (
	"time"

	"github.com/google/uuid"
)

// DocumentStatus represents the status of a document in the workflow.
type DocumentStatus string

const (
	DocumentStatusUploaded  DocumentStatus = "uploaded"
	DocumentStatusPipelined DocumentStatus = "pipelined"
	DocumentStatusIndexed   DocumentStatus = "indexed"
	DocumentStatusFailed    DocumentStatus = "failed"
)

// Document represents an uploaded file managed by the system.
type Document struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	OwnerID     string         `json:"owner_id" db:"owner_id"` // Can be system or user ID
	Name        string         `json:"name" db:"name"`
	S3Key       string         `json:"s3_key" db:"s3_key"`
	ContentType string         `json:"content_type" db:"content_type"`
	Size        int64          `json:"size" db:"size"`
	Status      DocumentStatus `json:"status" db:"status"`
	IsDir       bool           `json:"is_dir" db:"is_dir"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}
