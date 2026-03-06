// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"context"
	"database/sql"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"sync"
	"time"

	pgstore "school-with-ai/infra/eventstore/postgres"

	"github.com/jmoiron/sqlx"
)

// advisoryLockID is a fixed int64 used as the PostgreSQL advisory lock key.
// Only one connection across all pods can hold this lock at a time, ensuring
// a single projection updater leader.
const advisoryLockID int64 = 7_301_924_817 // arbitrary unique constant

// EventRouter defines the interface for dispatching events.
type EventRouter interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

// ProjectionUpdater polls the Postgres event store for new events and dispatches
// them to registered projections via a router using a pull-based polling loop.
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
func (u *ProjectionUpdater) pollLoop(ctx context.Context) error {
	// Get last checkpoint position
	lastPosition, err := u.checkpointStore.GetGlobalPosition(ctx, "all_projections")
	if err != nil {
		u.log.Info(ctx, "no checkpoint found, starting from beginning")
		lastPosition = 0
	} else {
		u.log.Info(ctx, "resuming from checkpoint", "global_position", lastPosition)
	}

	backoff := u.pollInterval
	eventsSinceCheckpoint := 0
	const checkpointInterval = 100

	// saveCheckpoint saves the checkpoint using a fresh context (not the cancelled one).
	saveCheckpoint := func(position int64) {
		saveCtx, saveCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer saveCancel()
		if saveErr := u.checkpointStore.SaveGlobalPosition(saveCtx, "all_projections", position); saveErr != nil {
			u.log.Error(saveCtx, "failed to save final checkpoint", "ERROR", saveErr)
		}
	}

	ticker := time.NewTicker(u.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Save final checkpoint before exit using a fresh context
			if eventsSinceCheckpoint > 0 {
				saveCheckpoint(lastPosition)
			}
			u.log.Info(ctx, "projection updater stopping due to context cancellation")
			return ctx.Err()

		case <-u.shutdownChan:
			if eventsSinceCheckpoint > 0 {
				saveCheckpoint(lastPosition)
			}
			u.log.Info(ctx, "projection updater stopping due to shutdown signal")
			return nil

		case <-ticker.C:
			// Poll for new events
			events, err := u.eventStore.ReadAllFromPosition(ctx, lastPosition, u.batchSize)
			if err != nil {
				u.log.Error(ctx, "failed to poll events", "ERROR", err)
				ticker.Reset(backoff)
				backoff = min(backoff*2, u.maxBackoff)
				continue
			}

			if len(events) == 0 {
				// No new events — idle backoff
				ticker.Reset(backoff)
				backoff = min(backoff*2, u.idleBackoff)
				continue
			}

			// Reset backoff on successful read
			backoff = u.pollInterval
			ticker.Reset(u.pollInterval)

			for _, ep := range events {
				if err := u.router.Dispatch(ctx, ep.Event); err != nil {
					u.log.Error(ctx, "failed to dispatch event",
						"eventType", ep.EventType,
						"globalPosition", ep.GlobalPosition,
						"ERROR", err)
				}

				lastPosition = ep.GlobalPosition
				eventsSinceCheckpoint++

				// Save checkpoint periodically
				if eventsSinceCheckpoint >= checkpointInterval {
					if saveErr := u.checkpointStore.SaveGlobalPosition(ctx, "all_projections", lastPosition); saveErr != nil {
						u.log.Error(ctx, "failed to save checkpoint", "ERROR", saveErr)
					} else {
						u.log.Info(ctx, "checkpoint saved", "global_position", lastPosition)
					}
					eventsSinceCheckpoint = 0
				}
			}
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
