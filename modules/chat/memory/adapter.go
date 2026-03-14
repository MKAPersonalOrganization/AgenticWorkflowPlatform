package memory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"school-with-ai/platform/artifact"
)

const (
	// MemorySessionID identifies the session used for storing long-term memory.
	// It is distinct from thread IDs to ensure separation.
	MemorySessionID = "long_term_memory"

	// MemoryFileName is the name of the markdown file storing the memories.
	MemoryFileName = "MEMORIES.md"

	// maxMemorySize caps the memory file read to 2 MB to prevent OOM.
	maxMemorySize = 2 << 20
)

// ArtifactMemoryService implements the Service interface using the Artifact Store as backend.
type ArtifactMemoryService struct {
	store artifact.Service
}

// NewArtifactMemoryService creates a new memory service backed by the artifact store.
func NewArtifactMemoryService(store artifact.Service) *ArtifactMemoryService {
	return &ArtifactMemoryService{store: store}
}

// Load retrieves memory for the user.
func (s *ArtifactMemoryService) Load(ctx context.Context, userID string) (string, error) {
	rc, _, err := s.store.Load(ctx, userID, MemorySessionID, MemoryFileName)
	if err != nil {
		if errors.Is(err, artifact.ErrNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("failed to load memory from artifact store: %w", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(io.LimitReader(rc, maxMemorySize))
	if err != nil {
		return "", fmt.Errorf("failed to read memory content: %w", err)
	}

	return string(content), nil
}

// Save persists memory for the user.
func (s *ArtifactMemoryService) Save(ctx context.Context, userID string, content string) error {
	_, err := s.store.Save(ctx, userID, MemorySessionID, MemoryFileName, "text/plain", strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("failed to save memory to artifact store: %w", err)
	}
	return nil
}
