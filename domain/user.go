package domain

import (
	"context"
	"errors"
	"time"

	client "github.com/ory/client-go"
)

var (
	// ErrIdentityNil is returned when an identity is nil.
	ErrIdentityNil = errors.New("identity cannot be nil")
	// ErrSessionNil is returned when a session or session identity is nil.
	ErrSessionNil = errors.New("session or session identity cannot be nil")
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const (
	// UserContextKey is the context key for storing the user.
	UserContextKey contextKey = "user"
	// SessionContextKey is the context key for storing the Kratos session.
	SessionContextKey contextKey = "session"
)

// IdentityTraits represents the user's identity traits as defined in the Kratos identity schema.
type IdentityTraits struct {
	Email string     `json:"email"`
	Name  NameTraits `json:"name"`
	Role  string     `json:"role"`
}

// NameTraits represents the user's name information.
type NameTraits struct {
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
}

// User represents a user in the system, combining Kratos identity with application data.
type User struct {
	ID        string         `json:"id"`         // Kratos identity ID
	Email     string         `json:"email"`      // Primary email (from traits)
	FirstName string         `json:"first_name"` // First name (from traits)
	LastName  string         `json:"last_name"`  // Last name (from traits)
	Role      string         `json:"role"`       // Role (from traits)
	CreatedAt time.Time      `json:"created_at"` // When identity was created
	UpdatedAt time.Time      `json:"updated_at"` // When identity was last updated
	Traits    IdentityTraits `json:"traits"`     // Full traits object
}

// NewUserFromIdentity creates a User from a Kratos Identity.
func NewUserFromIdentity(identity *client.Identity) (*User, error) {
	if identity == nil {
		return nil, ErrIdentityNil
	}

	user := &User{
		ID: identity.Id,
	}

	// Set timestamps (handle pointer types)
	if identity.CreatedAt != nil {
		user.CreatedAt = *identity.CreatedAt
	}
	if identity.UpdatedAt != nil {
		user.UpdatedAt = *identity.UpdatedAt
	}

	// Extract traits
	if traits, ok := identity.Traits.(map[string]any); ok {
		// Extract email
		if email, ok := traits["email"].(string); ok {
			user.Email = email
			user.Traits.Email = email
		}

		// Extract name
		if nameObj, ok := traits["name"].(map[string]any); ok {
			if first, ok := nameObj["first"].(string); ok {
				user.FirstName = first
				user.Traits.Name.First = first
			}
			if last, ok := nameObj["last"].(string); ok {
				user.LastName = last
				user.Traits.Name.Last = last
			}
		}

		// Extract role
		if role, ok := traits["role"].(string); ok {
			user.Role = role
			user.Traits.Role = role
		} else {
			// Default to user if not present
			user.Role = "user"
			user.Traits.Role = "user"
		}
	}

	return user, nil
}

// NewUserFromSession creates a User from a Kratos Session.
func NewUserFromSession(session *client.Session) (*User, error) {
	if session == nil || session.Identity == nil {
		return nil, ErrSessionNil
	}

	return NewUserFromIdentity(session.Identity)
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

// IsEmailVerified checks if the user's email is verified
// This requires checking the verifiable_addresses in the identity.
func (u *User) IsEmailVerified() bool {
	// This will be implemented when we have access to verifiable addresses
	// For now, return false as a safe default
	return false
}

// GetUserFromContext retrieves the authenticated user from the request context.
func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}

	return nil
}

// GetSessionFromContext retrieves the Kratos session from the request context.
func GetSessionFromContext(ctx context.Context) *client.Session {
	if session, ok := ctx.Value(SessionContextKey).(*client.Session); ok {
		return session
	}

	return nil
}

// IsAuthenticated checks if the context has an authenticated user.
func IsAuthenticated(ctx context.Context) bool {
	return GetUserFromContext(ctx) != nil
}

// HasName checks if the user has provided their name.
func (u *User) HasName() bool {
	return u.FirstName != "" || u.LastName != ""
}

// IsComplete checks if the user profile has all recommended information.
func (u *User) IsComplete() bool {
	return u.Email != "" && u.FirstName != "" && u.LastName != ""
}
