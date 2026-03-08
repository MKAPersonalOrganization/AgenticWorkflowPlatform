package shared

import (
	"context"
	"time"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for storing the user.
	UserContextKey contextKey = "user"

	// ThreadIDContextKey is the context key for storing the current thread ID.
	// Set by the engine before calling AgentProvider.GetConfig so that per-thread
	// state (e.g. activated skills) can be injected into the agent config.
	ThreadIDContextKey contextKey = "thread_id"
)

// User represents a user in the system, stored in the local database.
type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	GoogleSub string    `json:"google_sub" db:"google_sub"`
	FirstName string    `json:"first_name" db:"first_name"`
	LastName  string    `json:"last_name" db:"last_name"`
	Picture   string    `json:"picture" db:"picture"`
	Role      string    `json:"role" db:"role"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// GetFullName returns the user's full name (first + last).
func (u *User) GetFullName() string {
	if u.FirstName != "" && u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	if u.LastName != "" {
		return u.LastName
	}

	return u.Email
}

// GetDisplayName returns a name suitable for display (full name or email).
func (u *User) GetDisplayName() string {
	fullName := u.GetFullName()
	if fullName != u.Email {
		return fullName
	}

	return u.Email
}

// HasName checks if the user has provided their name.
func (u *User) HasName() bool {
	return u.FirstName != "" || u.LastName != ""
}

// GetUserFromContext retrieves the authenticated user from the request context.
func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}

	return nil
}

// SetUserInContext stores the user in the given context.
func SetUserInContext(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// GetThreadIDFromContext retrieves the current thread ID from the context.
func GetThreadIDFromContext(ctx context.Context) string {
	if tid, ok := ctx.Value(ThreadIDContextKey).(string); ok {
		return tid
	}
	return ""
}

// SetThreadIDInContext stores the thread ID in the given context.
func SetThreadIDInContext(ctx context.Context, threadID string) context.Context {
	return context.WithValue(ctx, ThreadIDContextKey, threadID)
}

// IsAuthenticated checks if the context has an authenticated user.
func IsAuthenticated(ctx context.Context) bool {
	return GetUserFromContext(ctx) != nil
}
