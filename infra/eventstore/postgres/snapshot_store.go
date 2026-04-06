package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	es "aicompass/foundation/eventsourcing"

	"github.com/jmoiron/sqlx"
)

// SnapshotStore implements es.SnapshotStore backed by PostgreSQL.
// Uses UPSERT so only the latest snapshot per stream is kept.
type SnapshotStore struct {
	db *sqlx.DB
}

// NewSnapshotStore creates a new PostgreSQL-backed snapshot store.
func NewSnapshotStore(db *sqlx.DB) *SnapshotStore {
	return &SnapshotStore{db: db}
}

// LoadSnapshot returns the most recent snapshot for the given stream.
// Returns nil, nil if no snapshot exists.
func (s *SnapshotStore) LoadSnapshot(ctx context.Context, streamName string) (*es.Snapshot, error) {
	var snap struct {
		StreamName string          `db:"stream_name"`
		Version    int64           `db:"version"`
		Data       json.RawMessage `db:"data"`
	}
	err := s.db.QueryRowxContext(ctx,
		`SELECT stream_name, version, data FROM aggregate_snapshots WHERE stream_name = $1`,
		streamName,
	).StructScan(&snap)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No snapshot yet — not an error.
		}
		return nil, err
	}
	return &es.Snapshot{
		StreamName: snap.StreamName,
		Version:    snap.Version,
		Data:       snap.Data,
	}, nil
}

// SaveSnapshot persists a snapshot for the given stream.
// Uses UPSERT to keep only the latest snapshot per stream.
func (s *SnapshotStore) SaveSnapshot(ctx context.Context, snapshot *es.Snapshot) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO aggregate_snapshots (stream_name, version, data, created_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (stream_name) DO UPDATE
		 SET version = EXCLUDED.version, data = EXCLUDED.data, created_at = NOW()`,
		snapshot.StreamName, snapshot.Version, snapshot.Data,
	)
	return err
}
