// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// CheckpointPosition represents a position in the event stream.
type CheckpointPosition struct {
	CommitPosition  uint64
	PreparePosition uint64
}

// CheckpointStore manages checkpoint positions for projection catch-up subscriptions.
type CheckpointStore struct {
	db *sqlx.DB
}

// NewCheckpointStore creates a new checkpoint store.
func NewCheckpointStore(db *sqlx.DB) *CheckpointStore {
	return &CheckpointStore{db: db}
}

// Get retrieves the last checkpoint position for a projection.
func (s *CheckpointStore) Get(ctx context.Context, projectionName string) (CheckpointPosition, error) {
	var result struct {
		CommitPosition  uint64 `db:"commit_position"`
		PreparePosition uint64 `db:"prepare_position"`
	}

	query := `
		SELECT commit_position, prepare_position
		FROM projection_checkpoints
		WHERE projection_name = $1
	`

	err := s.db.GetContext(ctx, &result, query, projectionName)
	if err != nil {
		return CheckpointPosition{}, fmt.Errorf("failed to get checkpoint: %w", err)
	}

	return CheckpointPosition{
		CommitPosition:  result.CommitPosition,
		PreparePosition: result.PreparePosition,
	}, nil
}

// Save saves the checkpoint position for a projection.
func (s *CheckpointStore) Save(ctx context.Context, projectionName string, position CheckpointPosition) error {
	query := `
		INSERT INTO projection_checkpoints (projection_name, commit_position, prepare_position, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (projection_name)
		DO UPDATE SET
			commit_position = EXCLUDED.commit_position,
			prepare_position = EXCLUDED.prepare_position,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		projectionName,
		position.CommitPosition,
		position.PreparePosition,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	return nil
}
