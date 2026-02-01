package core

import (
	"context"
	"time"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for storing the user.
	UserContextKey contextKey = "user"
)

// User represents a user in the system.
type User struct {
	ID        string    `json:"id"`         // User identity ID
	Email     string    `json:"email"`      // Primary email
	FirstName string    `json:"first_name"` // First name
	LastName  string    `json:"last_name"`  // Last name
	CreatedAt time.Time `json:"created_at"` // When identity was created
	UpdatedAt time.Time `json:"updated_at"` // When identity was last updated
}

// GetUserFromContext retrieves the user from the context.
func GetUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserContextKey).(*User)
	return user, ok
}

// WithUser adds a user to the context.
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}
