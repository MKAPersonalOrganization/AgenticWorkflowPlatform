package memory

import (
	"context"
	"errors"
	"fmt"
	"io"
	"school-with-ai/services/artifact"
	"strings"
)

const (
	// MemorySessionID identifies the session used for storing long-term memory.
	// It is distinct from thread IDs to ensure separation.
	MemorySessionID = "long_term_memory"

	// LegacyMemorySessionID is the old session ID that was used before migration.
	LegacyMemorySessionID = "user_shared"

	// MemoryFileName is the name of the markdown file storing the memories.
	MemoryFileName = "SCHOOLAI.md"

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
			// Fallback to legacy location for backward compatibility
			legacyRC, _, legacyErr := s.store.Load(ctx, userID, LegacyMemorySessionID, MemoryFileName)
			if legacyErr != nil {
				if errors.Is(legacyErr, artifact.ErrNotFound) {
					return "", nil
				}
				return "", fmt.Errorf("failed to load memory from legacy artifact store: %w", legacyErr)
			}
			defer legacyRC.Close()

			content, err := io.ReadAll(io.LimitReader(legacyRC, maxMemorySize))
			if err != nil {
				return "", fmt.Errorf("failed to read legacy memory content: %w", err)
			}

			// Migrate to new location
			if len(content) > 0 {
				contentStr := string(content)
				if migErr := s.Save(ctx, userID, contentStr); migErr != nil {
					// Log but don't fail - we can still return the content
					// In a production environment, you might want to log this error properly
				}
			}

			return string(content), nil
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
