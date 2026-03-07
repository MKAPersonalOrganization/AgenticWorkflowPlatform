package memory

import "context"

// Service defines the interface for managing long-term memory storage.
// It ensures that memories are scoped to a specific user.
type Service interface {
	// Load retrieves the content of the long-term memory for a given user.
	// Returns the memory content as a string. If no memory exists, returns empty string.
	Load(ctx context.Context, userID string) (string, error)

	// Save persists the content to the long-term memory for a given user.
	// Overwrites any existing content.
	Save(ctx context.Context, userID string, content string) error
}
