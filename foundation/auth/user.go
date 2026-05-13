package auth

import (
	"context"
	"time"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

// User is the canonical authenticated-user model. Authentication providers
// (infra/googleauth, infra/microsoftauth, …) map their provider-specific
// profile shapes into this type via UserResolver / UserUpserter.
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

func (u *User) GetDisplayName() string {
	fullName := u.GetFullName()
	if fullName != u.Email {
		return fullName
	}

	return u.Email
}

func (u *User) HasName() bool {
	return u.FirstName != "" || u.LastName != ""
}

func GetUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value(UserContextKey).(*User); ok {
		return user
	}

	return nil
}

func SetUserInContext(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}
