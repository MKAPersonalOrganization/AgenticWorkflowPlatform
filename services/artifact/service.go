package artifact

import (
	"context"
	"errors"
	"io"
)

// Service is the interface for the Artifact Store.
type Service interface {
	// Save saves a file to the artifact store.
	// content is the data to be written.
	// Returns the size of the saved file or an error.
	Save(ctx context.Context, userID, sessionID, fileName string, content io.Reader) (int64, error)

	// Load reads a file from the artifact store.
	// Returns a ReadCloser that must be closed by the caller.
	Load(ctx context.Context, userID, sessionID, fileName string) (io.ReadCloser, error)

	// List returns a list of filenames for a given session.
	List(ctx context.Context, userID, sessionID string) ([]string, error)

	// Delete removes a file from the artifact store.
	Delete(ctx context.Context, userID, sessionID, fileName string) error
}

var (
	// ErrNotFound is returned when an artifact is not found.
	ErrNotFound = errors.New("artifact not found")
	// ErrInvalidPath is returned when a path is invalid (e.g. traversal attempt).
	ErrInvalidPath = errors.New("invalid artifact path")
)
