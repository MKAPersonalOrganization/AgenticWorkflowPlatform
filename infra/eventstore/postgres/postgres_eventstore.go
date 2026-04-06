// Package postgres provides a PostgreSQL-backed implementation of es.EventStore.
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
	"strings"

	es "aicompass/foundation/eventsourcing"

	"github.com/jmoiron/sqlx"
)

// PostgresEventStore implements es.EventStore using PostgreSQL.
type PostgresEventStore struct {
	db       *sqlx.DB
	registry *es.DeserializerRegistry
}

// NewPostgresEventStore creates a new PostgreSQL-backed event store.
// The db connection should already be configured with proper pool settings.
func NewPostgresEventStore(db *sqlx.DB, registry *es.DeserializerRegistry) *PostgresEventStore {
	return &PostgresEventStore{db: db, registry: registry}
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
func (s *PostgresEventStore) AppendWithVersion(ctx context.Context, streamID string, expectedVersion int64, events ...es.Event) (int64, error) {
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
			return 0, es.ErrConcurrencyConflict
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
			return 0, es.ErrConcurrencyConflict
		}
		if expectedVersion >= 0 && currentVersion != expectedVersion {
			return 0, es.ErrConcurrencyConflict
		}
	}

	// Append events using a single batch INSERT for performance
	newVersion := currentVersion
	if len(events) == 1 {
		// Single event - simple INSERT
		newVersion++
		data, err := json.Marshal(events[0])
		if err != nil {
			return 0, fmt.Errorf("marshal event: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO events (stream_id, stream_version, event_type, data)
			 VALUES ($1, $2, $3, $4)`,
			streamID, newVersion, events[0].EventType(), data); err != nil {
			return 0, fmt.Errorf("insert event: %w", err)
		}
	} else {
		// Multiple events - batch INSERT to minimize round trips while holding advisory lock
		valueStrings := make([]string, 0, len(events))
		valueArgs := make([]interface{}, 0, len(events)*4)
		for i, event := range events {
			newVersion++
			data, err := json.Marshal(event)
			if err != nil {
				return 0, fmt.Errorf("marshal event %d: %w", i, err)
			}
			paramBase := i * 4
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)",
				paramBase+1, paramBase+2, paramBase+3, paramBase+4))
			valueArgs = append(valueArgs, streamID, newVersion, event.EventType(), data)
		}
		query := fmt.Sprintf(
			`INSERT INTO events (stream_id, stream_version, event_type, data) VALUES %s`,
			joinStrings(valueStrings, ", "))
		if _, err := tx.ExecContext(ctx, query, valueArgs...); err != nil {
			return 0, fmt.Errorf("batch insert events: %w", err)
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
func (s *PostgresEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]es.Event, error) {
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

	var events []es.Event
	for rows.Next() {
		var eventType string
		var data []byte
		if err := rows.Scan(&eventType, &data); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		event, err := DeserializeDomainEvent(s.registry, eventType, data)
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

		event, err := DeserializeDomainEvent(s.registry, ep.EventType, data)
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

// Verify PostgresEventStore implements es.EventStore.
var _ es.EventStore = (*PostgresEventStore)(nil)

// joinStrings joins a slice of strings with a separator (avoids importing strings just for Join).
func joinStrings(elems []string, sep string) string {
	return strings.Join(elems, sep)
}
