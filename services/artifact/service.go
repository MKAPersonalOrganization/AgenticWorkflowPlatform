package artifact

import (
	"context"
	"errors"
	"io"
)

// ArtifactMeta contains metadata about a stored artifact.
type ArtifactMeta struct {
	ContentType string
	Size        int64
}

// Service is the interface for the Artifact Store.
type Service interface {
	// Save saves a file to the artifact store.
	// contentType is the MIME type (e.g. "audio/wav", "image/png").
	// An empty contentType defaults to "application/octet-stream".
	// Returns the size of the saved file or an error.
	Save(ctx context.Context, userID, sessionID, fileName, contentType string, content io.Reader) (int64, error)

	// Load reads a file from the artifact store.
	// Returns a ReadCloser that must be closed by the caller, metadata, or an error.
	Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, ArtifactMeta, error)

	// List returns a list of filenames for a given session.
	List(ctx context.Context, userID, sessionID string) ([]string, error)

	// Delete removes a file from the artifact store.
	Delete(ctx context.Context, userID, sessionID, fileName string) error

	// HealthCheck verifies the storage backend is reachable.
	// Used by readiness probes to detect S3/MinIO outages.
	HealthCheck(ctx context.Context) error
}

var (
	// ErrNotFound is returned when an artifact is not found.
	ErrNotFound = errors.New("artifact not found")
	// ErrInvalidPath is returned when a path is invalid (e.g. traversal attempt).
	ErrInvalidPath = errors.New("invalid artifact path")
)
