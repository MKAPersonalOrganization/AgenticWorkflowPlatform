package agent

import (
	"context"
)

// Artifacts provides access to artifact storage for the session.
// Artifacts are user-uploaded or agent-generated files associated with a session.
type Artifacts interface {
	// Save stores an artifact with the given name.
	Save(ctx context.Context, name string, data []byte, mimeType string) error

	// Load retrieves an artifact by name.
	Load(ctx context.Context, name string) ([]byte, string, error)

	// List returns all artifact names.
	List(ctx context.Context) ([]string, error)
}
