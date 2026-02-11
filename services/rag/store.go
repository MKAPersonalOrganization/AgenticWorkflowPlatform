package rag

import (
	"context"
	"io"
	"time"
)

// FileInfo represents a file or directory in the RAG storage.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// Store defines the interface for RAG document storage.
type Store interface {
	// Save saves a file to the storage.
	Save(ctx context.Context, scope Scope, ownerID, filename string, content io.Reader) (int64, error)

	// CreateDirectory creates a directory in the storage.
	CreateDirectory(ctx context.Context, scope Scope, ownerID, path string) error

	// Load loads a file from the storage.
	Load(ctx context.Context, scope Scope, ownerID, filename string) (io.ReadCloser, error)

	// List lists files in the workspace with metadata.
	List(ctx context.Context, scope Scope, ownerID, prefix string) ([]FileInfo, error)

	// Delete deletes a file from the storage.
	Delete(ctx context.Context, scope Scope, ownerID, filename string) error
}
