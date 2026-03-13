// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	es "school-with-ai/foundation/eventsourcing"
	"school-with-ai/foundation/logger"

	pgstore "school-with-ai/infra/eventstore/postgres"

	"github.com/jmoiron/sqlx"
)

// advisoryLockID is a fixed int64 used as the PostgreSQL advisory lock key.
// Only one connection across all pods can hold this lock at a time, ensuring
// a single projection updater leader.
const advisoryLockID int64 = 7_301_924_817 // arbitrary unique constant

// EventRouter defines the interface for dispatching events.
type EventRouter interface {
	Dispatch(ctx context.Context, event es.Event) error
}

// ProjectionUpdater polls the Postgres event store for new events and dispatches
// them to registered projections via a router using a pull-based polling loop.
//
// Per-projection checkpoints: Each registered projection name has its own
// checkpoint row. The updater polls from the minimum checkpoint across all
// projections, and after successfully dispatching a batch it advances each
// projection's checkpoint to the latest processed position. To rebuild a
// single projection, reset its checkpoint to 0 without affecting others.
//
// Leader election: In a multi-pod deployment every pod starts a ProjectionUpdater,
// but only the pod that holds the PostgreSQL session-level advisory lock
// (pg_try_advisory_lock) actually polls and processes events. Other pods retry
// the lock acquisition every leaderRetryInterval. When the leader pod shuts down
// or loses its database connection the lock is automatically released and another
// pod takes over.
type ProjectionUpdater struct {
	log             *logger.Logger
	db              *sqlx.DB
	eventStore      *pgstore.PostgresEventStore
	checkpointStore *CheckpointStore
	router          EventRouter
	shutdownChan    chan struct{}
	shutdownOnce    sync.Once
	projectionNames []string // registered projection names for per-projection checkpoints

	// Polling configuration
	pollInterval        time.Duration
	batchSize           int
	idleBackoff         time.Duration
	maxBackoff          time.Duration
	leaderRetryInterval time.Duration
}

// ProjectionUpdaterOption configures the projection updater.
type ProjectionUpdaterOption func(*ProjectionUpdater)

// WithPollInterval sets the polling interval (default 100ms).
func WithPollInterval(d time.Duration) ProjectionUpdaterOption {
	return func(u *ProjectionUpdater) { u.pollInterval = d }
}

// WithBatchSize sets the max events per poll (default 500).
func WithBatchSize(n int) ProjectionUpdaterOption {
	return func(u *ProjectionUpdater) { u.batchSize = n }
}

// WithLeaderRetryInterval sets how often a non-leader pod retries lock
// acquisition (default 5s).
func WithLeaderRetryInterval(d time.Duration) ProjectionUpdaterOption {
	return func(u *ProjectionUpdater) { u.leaderRetryInterval = d }
}

// NewProjectionUpdater creates a new projection updater that polls Postgres.
func NewProjectionUpdater(
	log *logger.Logger,
	eventStore *pgstore.PostgresEventStore,
	db *sqlx.DB,
	router EventRouter,
	opts ...ProjectionUpdaterOption,
) *ProjectionUpdater {
	u := &ProjectionUpdater{
		log:                 log,
		db:                  db,
		eventStore:          eventStore,
		checkpointStore:     NewCheckpointStore(db),
		router:              router,
		shutdownChan:        make(chan struct{}),
		pollInterval:        100 * time.Millisecond,
		batchSize:           500,
		idleBackoff:         500 * time.Millisecond,
		maxBackoff:          5 * time.Second,
		leaderRetryInterval: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// RegisterProjectionName adds a named projection so the updater tracks a
// separate checkpoint for it. Must be called before Start.
func (u *ProjectionUpdater) RegisterProjectionName(name string) {
	u.projectionNames = append(u.projectionNames, name)
}

// Start begins the leader election + polling loop. This is a blocking call.
//
// Leader election strategy:
//  1. Acquire a dedicated *sql.Conn from the pool (advisory locks are session-scoped).
//  2. Try pg_try_advisory_lock(advisoryLockID) — non-blocking.
//  3. If acquired → run the polling loop on that connection.
//     If not acquired → sleep leaderRetryInterval and retry.
//  4. On context cancellation or shutdown the connection is closed, which
//     automatically releases the advisory lock and lets another pod take over.
func (u *ProjectionUpdater) Start(ctx context.Context) error {
	u.log.Info(ctx, "projection updater starting (leader election mode)")

	for {
		// Check for cancellation before attempting lock.
		select {
		case <-ctx.Done():
			u.log.Info(ctx, "projection updater stopping before acquiring leadership")
			return ctx.Err()
		case <-u.shutdownChan:
			u.log.Info(ctx, "projection updater stopping before acquiring leadership (shutdown)")
			return nil
		default:
		}

		// Try to become the leader.
		err := u.runAsLeader(ctx)
		if err != nil {
			// If the context was cancelled, exit cleanly.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			u.log.Error(ctx, "leader session ended", "ERROR", err)
		}

		// Wait before retrying lock acquisition.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-u.shutdownChan:
			return nil
		case <-time.After(u.leaderRetryInterval):
		}
	}
}

// tryAcquireAdvisoryLock attempts a non-blocking session-level advisory lock on
// the given raw *sql.Conn. Returns true if the lock was acquired.
func tryAcquireAdvisoryLock(ctx context.Context, conn *sql.Conn, lockID int64) (bool, error) {
	var acquired bool
	err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	return acquired, nil
}

// runAsLeader tries to acquire the advisory lock and, if successful, runs the
// polling loop until the context is cancelled, the shutdown channel fires, or the
// database connection is lost. The advisory lock is released when the *sql.Conn
// is closed.
func (u *ProjectionUpdater) runAsLeader(ctx context.Context) error {
	// Acquire a dedicated connection — advisory locks are bound to the PG session.
	conn, err := u.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring db connection for leader lock: %w", err)
	}
	defer conn.Close()

	acquired, err := tryAcquireAdvisoryLock(ctx, conn, advisoryLockID)
	if err != nil {
		return fmt.Errorf("trying advisory lock: %w", err)
	}
	if !acquired {
		u.log.Info(ctx, "projection updater: another pod is leader, waiting")
		return nil // not an error — will retry
	}

	u.log.Info(ctx, "projection updater: acquired leadership lock, starting poll loop")

	err = u.pollLoop(ctx)

	u.log.Info(ctx, "projection updater: releasing leadership lock")
	return err
}

// pollLoop is the core event-processing loop, extracted so it only runs while
// this pod holds the advisory lock.
//
// Per-projection checkpoints: the updater maintains a separate checkpoint row
// per registered projection name. It polls from the minimum position across
// all projections and advances each projection's checkpoint after a successful
// dispatch batch.
func (u *ProjectionUpdater) pollLoop(ctx context.Context) error {
	names := u.projectionNames
	if len(names) == 0 {
		return fmt.Errorf("projection updater: no projection names registered")
	}

	// Load the checkpoint for each registered projection.
	checkpoints := make(map[string]int64, len(names))
	minPosition := int64(0)
	first := true
	for _, name := range names {
		pos, err := u.checkpointStore.GetGlobalPosition(ctx, name)
		if err != nil {
			u.log.Info(ctx, "no checkpoint found for projection, starting from 0", "projection", name)
			pos = 0
		} else {
			u.log.Info(ctx, "resuming projection from checkpoint", "projection", name, "global_position", pos)
		}
		checkpoints[name] = pos
		if first || pos < minPosition {
			minPosition = pos
			first = false
		}
	}

	backoff := u.pollInterval
	eventsSinceCheckpoint := 0
	const checkpointInterval = 100

	// saveCheckpoints saves every projection's checkpoint using a fresh context.
	saveCheckpoints := func() {
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		for _, name := range names {
			if saveErr := u.checkpointStore.SaveGlobalPosition(saveCtx, name, checkpoints[name]); saveErr != nil {
				u.log.Error(saveCtx, "failed to save final checkpoint", "projection", name, "ERROR", saveErr)
			}
		}
	}

	ticker := time.NewTicker(u.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if eventsSinceCheckpoint > 0 {
				saveCheckpoints()
			}
			u.log.Info(ctx, "projection updater stopping due to context cancellation")
			return ctx.Err()

		case <-u.shutdownChan:
			if eventsSinceCheckpoint > 0 {
				saveCheckpoints()
			}
			u.log.Info(ctx, "projection updater stopping due to shutdown signal")
			return nil

		case <-ticker.C:
			events, err := u.eventStore.ReadAllFromPosition(ctx, minPosition, u.batchSize)
			if err != nil {
				u.log.Error(ctx, "failed to poll events", "ERROR", err)
				ticker.Reset(backoff)
				backoff = min(backoff*2, u.maxBackoff)
				continue
			}

			if len(events) == 0 {
				ticker.Reset(backoff)
				backoff = min(backoff*2, u.idleBackoff)
				continue
			}

			backoff = u.pollInterval
			ticker.Reset(u.pollInterval)

			for _, ep := range events {
				if err := u.router.Dispatch(ctx, ep.Event); err != nil {
					u.log.Error(ctx, "failed to dispatch event",
						"eventType", ep.EventType,
						"globalPosition", ep.GlobalPosition,
						"ERROR", err)
				}

				// Advance every projection's checkpoint to the latest position.
				for _, name := range names {
					if ep.GlobalPosition > checkpoints[name] {
						checkpoints[name] = ep.GlobalPosition
					}
				}
				eventsSinceCheckpoint++

				// Periodic checkpoint save.
				if eventsSinceCheckpoint >= checkpointInterval {
					for _, name := range names {
						if saveErr := u.checkpointStore.SaveGlobalPosition(ctx, name, checkpoints[name]); saveErr != nil {
							u.log.Error(ctx, "failed to save checkpoint", "projection", name, "ERROR", saveErr)
						}
					}
					u.log.Info(ctx, "checkpoints saved", "min_position", minPosition)
					eventsSinceCheckpoint = 0
				}
			}

			// Update minPosition for the next poll.
			newMin := checkpoints[names[0]]
			for _, name := range names[1:] {
				if checkpoints[name] < newMin {
					newMin = checkpoints[name]
				}
			}
			minPosition = newMin
		}
	}
}

// Shutdown signals the updater to stop gracefully. Safe to call multiple times.
func (u *ProjectionUpdater) Shutdown() {
	u.shutdownOnce.Do(func() {
		close(u.shutdownChan)
	})
}

// min returns the smaller of two durations.
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// WaitForEvent blocks until at least one event beyond afterPosition is available,
// or the context is cancelled. Useful for tests.
func (u *ProjectionUpdater) WaitForEvent(ctx context.Context, afterPosition int64) error {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			events, err := u.eventStore.ReadAllFromPosition(ctx, afterPosition, 1)
			if err != nil {
				return fmt.Errorf("wait for event: %w", err)
			}
			if len(events) > 0 {
				return nil
			}
		}
	}
}
