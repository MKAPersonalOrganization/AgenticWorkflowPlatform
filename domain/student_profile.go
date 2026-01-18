package domain

import (
	"context"
	"time"
)

// StudentProfile represents the extended academic profile of a student.
// This is separate from the User identity (Kratos).
type StudentProfile struct {
	UserID        string    `json:"user_id" db:"user_id"`
	LearningStyle string    `json:"learning_style" db:"learning_style"`
	Struggles     []string  `json:"struggles" db:"struggles"`
	Strengths     []string  `json:"strengths" db:"strengths"`
	Goals         string    `json:"goals" db:"goals"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// StudentProfileRepository defines the interface for extending student storage.
type StudentProfileRepository interface {
	GetByUserID(ctx context.Context, userID string) (*StudentProfile, error)
	Upsert(ctx context.Context, profile *StudentProfile) error
}
