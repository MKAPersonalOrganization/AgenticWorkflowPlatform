package domain

import (
	"context"
	"time"
)

// StudentProfile represents the extended academic profile of a student.
// This is separate from the User identity (Kratos).
//
// This type lives in domain/ (shared kernel) because it is consumed across
// multiple layers: modules/identity/ (owner), modules/chat/, and
// foundation/agentsdk/middleware/. The import cycle that previously prevented
// moving it (components/profile → domain) was broken in ADR 016.
type StudentProfile struct {
	UserID        string    `json:"user_id" db:"user_id"`
	LearningStyle string    `json:"learning_style" db:"learning_style"`
	Struggles     []string  `json:"struggles" db:"struggles"`
	Strengths     []string  `json:"strengths" db:"strengths"`
	Goals         string    `json:"goals" db:"goals"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// StudentProfileRepository defines the interface for student profile storage.
type StudentProfileRepository interface {
	GetByUserID(ctx context.Context, userID string) (*StudentProfile, error)
	Upsert(ctx context.Context, profile *StudentProfile) error
}
