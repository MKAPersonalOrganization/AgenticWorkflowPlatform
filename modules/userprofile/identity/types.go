package identity

import (
	"context"
	"time"
)

// UserProfile represents the extended profile of a user.
// This is separate from the User identity (Kratos).
type UserProfile struct {
	UserID        string    `json:"user_id" db:"user_id"`
	LearningStyle string    `json:"learning_style" db:"learning_style"`
	Struggles     []string  `json:"struggles" db:"struggles"`
	Strengths     []string  `json:"strengths" db:"strengths"`
	Goals         string    `json:"goals" db:"goals"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// UserProfileRepository defines the interface for user profile storage.
type UserProfileRepository interface {
	GetByUserID(ctx context.Context, userID string) (*UserProfile, error)
	Upsert(ctx context.Context, profile *UserProfile) error
}
