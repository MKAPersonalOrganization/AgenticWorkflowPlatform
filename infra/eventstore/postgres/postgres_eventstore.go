// Package postgres provides a PostgreSQL-backed implementation of domain.EventStore.
// It uses advisory locks for stream-level concurrency, JSONB for event data,
// and a global sequence (global_position) for ordered event processing.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"

	"school-with-ai/domain"

	"github.com/jmoiron/sqlx"
)

// PostgresEventStore implements domain.EventStore using PostgreSQL.
type PostgresEventStore struct {
	db *sqlx.DB
}

// NewPostgresEventStore creates a new PostgreSQL-backed event store.
// The db connection should already be configured with proper pool settings.
func NewPostgresEventStore(db *sqlx.DB) *PostgresEventStore {
	return &PostgresEventStore{db: db}
}

// advisoryLockKey generates a deterministic advisory lock key from a stream ID.
func advisoryLockKey(streamID string) int64 {
	h := fnv.New64a()
	h.Write([]byte("eventstore:" + streamID))
	return int64(h.Sum64()) // Safe: advisory locks accept any int64
}

// AppendWithVersion adds events to a stream with optimistic concurrency control.
// It uses an advisory lock per stream to serialize appends and prevent conflicts.
//
// expectedVersion semantics:
//
//	>= 0  : exact version check (optimistic concurrency)
//	-2     : NoStream - stream must NOT exist
//	-1     : Any - no concurrency check
func (s *PostgresEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...domain.Event) (int64, error) {
	if len(events) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Acquire advisory lock for this stream within the transaction
	lockKey := advisoryLockKey(streamID)
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return 0, fmt.Errorf("acquire advisory lock: %w", err)
	}

	// Get or create stream, check expected version
	var currentVersion int64
	err = tx.GetContext(ctx, &currentVersion,
		"SELECT stream_version FROM event_streams WHERE stream_id = $1 FOR UPDATE", streamID)

	if errors.Is(err, sql.ErrNoRows) {
		// Stream doesn't exist
		if expectedVersion >= 0 {
			// Caller expected a specific version but stream doesn't exist
			return 0, domain.ErrConcurrencyConflict
		}
		// Create the stream (NoStream or Any)
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO event_streams (stream_id, stream_version) VALUES ($1, 0)",
			streamID); err != nil {
			return 0, fmt.Errorf("create stream: %w", err)
		}
		currentVersion = 0
	} else if err != nil {
		return 0, fmt.Errorf("get stream version: %w", err)
	} else {
		// Stream exists
		if expectedVersion == -2 {
			// NoStream: stream must not exist
			return 0, domain.ErrConcurrencyConflict
		}
		if expectedVersion >= 0 && currentVersion != expectedVersion {
			return 0, domain.ErrConcurrencyConflict
		}
	}

	// Append events
	newVersion := currentVersion
	for _, event := range events {
		newVersion++

		data, err := json.Marshal(event)
		if err != nil {
			return 0, fmt.Errorf("marshal event: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO events (stream_id, stream_version, event_type, data)
			 VALUES ($1, $2, $3, $4)`,
			streamID, newVersion, event.EventType(), data); err != nil {
			return 0, fmt.Errorf("insert event: %w", err)
		}
	}

	// Update stream version
	if _, err := tx.ExecContext(ctx,
		"UPDATE event_streams SET stream_version = $1, updated_at = NOW() WHERE stream_id = $2",
		newVersion, streamID); err != nil {
		return 0, fmt.Errorf("update stream version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return newVersion, nil
}

// ReadStream reads all events for a stream starting from a specific version.
// Returns an empty slice if the stream doesn't exist (not an error).
func (s *PostgresEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]domain.Event, error) {
	if fromVersion < 0 {
		return nil, fmt.Errorf("invalid fromVersion: must be non-negative, got %d", fromVersion)
	}

	rows, err := s.db.QueryxContext(ctx,
		`SELECT event_type, data FROM events
		 WHERE stream_id = $1 AND stream_version > $2
		 ORDER BY stream_version ASC`,
		streamID, fromVersion)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		var eventType string
		var data []byte
		if err := rows.Scan(&eventType, &data); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		event, err := DeserializeDomainEvent(eventType, data)
		if err != nil {
			// Skip unknown event types (forward compatibility)
			continue
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

// DeleteStream removes a stream and all its events.
func (s *PostgresEventStore) DeleteStream(ctx context.Context, streamID string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Advisory lock to prevent concurrent operations on this stream
	lockKey := advisoryLockKey(streamID)
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}

	// Delete events (cascade from stream) and the stream itself
	if _, err := tx.ExecContext(ctx, "DELETE FROM event_streams WHERE stream_id = $1", streamID); err != nil {
		return fmt.Errorf("delete stream: %w", err)
	}

	return tx.Commit()
}

// Close is a no-op for PostgresEventStore since the db connection is managed externally.
func (s *PostgresEventStore) Close() error {
	return nil
}

// ReadAllFromPosition reads events across all streams starting from a global position.
// This is used by the projection updater to catch up on events.
// Returns the events and the highest global_position seen.
func (s *PostgresEventStore) ReadAllFromPosition(ctx context.Context, afterPosition int64, limit int) ([]EventWithPosition, error) {
	rows, err := s.db.QueryxContext(ctx,
		`SELECT global_position, stream_id, stream_version, event_type, data, created_at
		 FROM events
		 WHERE global_position > $1
		 ORDER BY global_position ASC
		 LIMIT $2`,
		afterPosition, limit)
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}
	defer rows.Close()

	var result []EventWithPosition
	for rows.Next() {
		var ep EventWithPosition
		var data []byte
		if err := rows.Scan(&ep.GlobalPosition, &ep.StreamID, &ep.StreamVersion, &ep.EventType, &data, &ep.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		event, err := DeserializeDomainEvent(ep.EventType, data)
		if err != nil {
			// Skip unknown event types
			continue
		}
		ep.Event = event
		result = append(result, ep)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return result, nil
}

// Verify PostgresEventStore implements domain.EventStore.
var _ domain.EventStore = (*PostgresEventStore)(nil)
