// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// CheckpointStore manages checkpoint positions for projection polling.
type CheckpointStore struct {
	db *sqlx.DB
}

// NewCheckpointStore creates a new checkpoint store.
func NewCheckpointStore(db *sqlx.DB) *CheckpointStore {
	return &CheckpointStore{db: db}
}

// GetGlobalPosition retrieves the last global_position checkpoint for a projection.
func (s *CheckpointStore) GetGlobalPosition(ctx context.Context, projectionName string) (int64, error) {
	var pos int64
	query := `SELECT global_position FROM projection_checkpoints WHERE projection_name = $1`
	err := s.db.GetContext(ctx, &pos, query, projectionName)
	if err != nil {
		return 0, fmt.Errorf("failed to get global position checkpoint: %w", err)
	}
	return pos, nil
}

// SaveGlobalPosition saves the global_position checkpoint for a projection.
func (s *CheckpointStore) SaveGlobalPosition(ctx context.Context, projectionName string, position int64) error {
	query := `
		INSERT INTO projection_checkpoints (projection_name, global_position, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (projection_name)
		DO UPDATE SET
			global_position = EXCLUDED.global_position,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query, projectionName, position, time.Now())
	if err != nil {
		return fmt.Errorf("failed to save global position checkpoint: %w", err)
	}
	return nil
}
