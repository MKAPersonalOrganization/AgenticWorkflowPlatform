package domain

// This file re-exports shared kernel types for backward compatibility.
// New code should import "school-with-ai/shared" directly.
//
// These aliases will be removed once all consumers have migrated.

import (
	"context"

	"school-with-ai/shared"
)

// User is a type alias to the shared kernel User.
type User = shared.User

// UserContextKey is re-exported from shared for backward compatibility.
var UserContextKey = shared.UserContextKey

// GetUserFromContext retrieves the authenticated user from the request context.
func GetUserFromContext(ctx context.Context) *User {
	return shared.GetUserFromContext(ctx)
}

// IsAuthenticated checks if the context has an authenticated user.
func IsAuthenticated(ctx context.Context) bool {
	return shared.IsAuthenticated(ctx)
}
