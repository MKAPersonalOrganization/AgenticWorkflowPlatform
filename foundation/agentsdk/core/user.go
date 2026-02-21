package core

import (
	"time"
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
