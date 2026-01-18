package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"school-with-ai/domain"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// PostgresStudentProfileRepository implements domain.StudentProfileRepository.
type PostgresStudentProfileRepository struct {
	db *sqlx.DB
}

// NewPostgresStudentProfileRepository creates a new repository.
func NewPostgresStudentProfileRepository(db *sqlx.DB) *PostgresStudentProfileRepository {
	return &PostgresStudentProfileRepository{db: db}
}

// GetByUserID retrieves a profile by User ID.
func (r *PostgresStudentProfileRepository) GetByUserID(ctx context.Context, userID string) (*domain.StudentProfile, error) {
	query := `
		SELECT user_id, learning_style, struggles, strengths, goals, created_at, updated_at
		FROM student_profiles
		WHERE user_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, userID)

	var profile domain.StudentProfile
	var struggles, strengths []string

	err := row.Scan(
		&profile.UserID,
		&profile.LearningStyle,
		(*pq.StringArray)(&struggles),
		(*pq.StringArray)(&strengths),
		&profile.Goals,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Return nil if not found, let caller handle
		}
		return nil, fmt.Errorf("failed to get student profile: %w", err)
	}

	profile.Struggles = struggles
	profile.Strengths = strengths

	return &profile, nil
}

// Upsert creates or updates a student profile.
func (r *PostgresStudentProfileRepository) Upsert(ctx context.Context, profile *domain.StudentProfile) error {
	query := `
		INSERT INTO student_profiles (user_id, learning_style, struggles, strengths, goals, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET
			learning_style = EXCLUDED.learning_style,
			struggles = EXCLUDED.struggles,
			strengths = EXCLUDED.strengths,
			goals = EXCLUDED.goals,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now

	_, err := r.db.ExecContext(ctx, query,
		profile.UserID,
		profile.LearningStyle,
		pq.StringArray(profile.Struggles),
		pq.StringArray(profile.Strengths),
		profile.Goals,
		profile.CreatedAt,
		profile.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert student profile: %w", err)
	}

	return nil
}
