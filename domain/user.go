package domain

import (
	"time"

	client "github.com/ory/client-go"
)

// IdentityTraits represents the user's identity traits as defined in the Kratos identity schema
type IdentityTraits struct {
	Email string     `json:"email"`
	Name  NameTraits `json:"name,omitempty"`
}

// NameTraits represents the user's name information
type NameTraits struct {
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
}

// User represents a user in the system, combining Kratos identity with application data
type User struct {
	ID        string         `json:"id"`         // Kratos identity ID
	Email     string         `json:"email"`      // Primary email (from traits)
	FirstName string         `json:"first_name"` // First name (from traits)
	LastName  string         `json:"last_name"`  // Last name (from traits)
	CreatedAt time.Time      `json:"created_at"` // When identity was created
	UpdatedAt time.Time      `json:"updated_at"` // When identity was last updated
	Traits    IdentityTraits `json:"traits"`     // Full traits object
}

// NewUserFromIdentity creates a User from a Kratos Identity
func NewUserFromIdentity(identity *client.Identity) (*User, error) {
	if identity == nil {
		return nil, nil
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
	if traits, ok := identity.Traits.(map[string]interface{}); ok {
		// Extract email
		if email, ok := traits["email"].(string); ok {
			user.Email = email
			user.Traits.Email = email
		}

		// Extract name
		if nameObj, ok := traits["name"].(map[string]interface{}); ok {
			if first, ok := nameObj["first"].(string); ok {
				user.FirstName = first
				user.Traits.Name.First = first
			}
			if last, ok := nameObj["last"].(string); ok {
				user.LastName = last
				user.Traits.Name.Last = last
			}
		}
	}

	return user, nil
}

// NewUserFromSession creates a User from a Kratos Session
func NewUserFromSession(session *client.Session) (*User, error) {
	if session == nil || session.Identity == nil {
		return nil, nil
	}
	return NewUserFromIdentity(session.Identity)
}

// GetFullName returns the user's full name (first + last)
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

// GetDisplayName returns a name suitable for display (full name or email)
func (u *User) GetDisplayName() string {
	fullName := u.GetFullName()
	if fullName != u.Email {
		return fullName
	}
	return u.Email
}

// IsEmailVerified checks if the user's email is verified
// This requires checking the verifiable_addresses in the identity
func (u *User) IsEmailVerified() bool {
	// This will be implemented when we have access to verifiable addresses
	// For now, return false as a safe default
	return false
}

// HasName checks if the user has provided their name
func (u *User) HasName() bool {
	return u.FirstName != "" || u.LastName != ""
}

// IsComplete checks if the user profile has all recommended information
func (u *User) IsComplete() bool {
	return u.Email != "" && u.FirstName != "" && u.LastName != ""
}
