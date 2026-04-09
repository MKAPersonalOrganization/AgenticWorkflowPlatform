package auth

import (
	"aicompass/shared"
	"context"
	"net/http"
)

// SessionReader retrieves the authenticated user's identity from an HTTP session.
type SessionReader interface {
	// GetUserID extracts the user ID stored in the session cookie.
	// Returns ("", false) when the request carries no valid session.
	GetUserID(r *http.Request) (string, bool)
}

// UserResolver loads a User by their unique identifier.
type UserResolver interface {
	GetUserByID(ctx context.Context, userID string) (*shared.User, error)
}
