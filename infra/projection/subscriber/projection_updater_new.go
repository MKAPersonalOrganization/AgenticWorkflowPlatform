// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"aicompass/foundation/logger"
	"context"
	"database/sql"
	"fmt"
	"time"

	es "aicompass/foundation/eventsourcing"

	pgstore "aicompass/infra/eventstore/postgres"

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
	defer func() { _ = conn.Close() }()

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

	checkpoints, minPosition := u.loadCheckpoints(ctx, names)
	eventsSinceCheckpoint := 0
	backoff := u.pollInterval

	saveCheckpoints := func() {
		u.saveAllCheckpoints(names, checkpoints)
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
			minPosition, backoff = u.pollTick(ctx, names, checkpoints, minPosition, &eventsSinceCheckpoint, backoff, ticker)
		}
	}
}

// loadCheckpoints fetches each projection's stored global position (or
// starts at 0 when missing) and returns the checkpoint map plus the
// minimum position across all projections — the starting point for the
// next event read.
func (u *ProjectionUpdater) loadCheckpoints(ctx context.Context, names []string) (map[string]int64, int64) {
	checkpoints := make(map[string]int64, len(names))
	var minPosition int64
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
	return checkpoints, minPosition
}

// saveAllCheckpoints persists every projection's current checkpoint
// using a fresh 5s-timeout context so shutdown flushes aren't blocked
// by a cancelled parent. Errors are logged, not returned.
func (u *ProjectionUpdater) saveAllCheckpoints(names []string, checkpoints map[string]int64) {
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, name := range names {
		if err := u.checkpointStore.SaveGlobalPosition(saveCtx, name, checkpoints[name]); err != nil {
			u.log.Error(saveCtx, "failed to save final checkpoint", "projection", name, "ERROR", err)
		}
	}
}

// pollTick runs one iteration of the ticker loop: read a batch, apply
// error/idle backoff, dispatch + checkpoint each event, and recompute
// minPosition for the next read. Returns the updated (minPosition,
// backoff). eventsSinceCheckpoint is a pointer because the caller
// tracks it across iterations for final-flush decisions.
func (u *ProjectionUpdater) pollTick(
	ctx context.Context, names []string, checkpoints map[string]int64,
	minPosition int64, eventsSinceCheckpoint *int, backoff time.Duration,
	ticker *time.Ticker,
) (int64, time.Duration) {
	events, err := u.eventStore.ReadAllFromPosition(ctx, minPosition, u.batchSize)
	if err != nil {
		u.log.Error(ctx, "failed to poll events", "ERROR", err)
		newBackoff := min(backoff*2, u.maxBackoff)
		ticker.Reset(newBackoff)
		return minPosition, newBackoff
	}
	if len(events) == 0 {
		newBackoff := min(backoff*2, u.idleBackoff)
		ticker.Reset(newBackoff)
		return minPosition, newBackoff
	}

	ticker.Reset(u.pollInterval)
	for _, ep := range events {
		u.dispatchAndCheckpoint(ctx, ep, names, checkpoints, eventsSinceCheckpoint, minPosition)
	}
	return minCheckpoint(names, checkpoints), u.pollInterval
}

// dispatchAndCheckpoint runs one event through the router and advances
// every projection's checkpoint. Periodically flushes checkpoints to
// the store (every checkpointInterval events) so a crash loses at most
// one batch of work.
func (u *ProjectionUpdater) dispatchAndCheckpoint(
	ctx context.Context, ep pgstore.EventWithPosition, names []string,
	checkpoints map[string]int64, eventsSinceCheckpoint *int, minPosition int64,
) {
	const checkpointInterval = 100
	if err := u.router.Dispatch(ctx, ep.Event); err != nil {
		u.log.Error(ctx, "failed to dispatch event",
			"eventType", ep.EventType,
			"globalPosition", ep.GlobalPosition,
			"ERROR", err)
	}
	for _, name := range names {
		if ep.GlobalPosition > checkpoints[name] {
			checkpoints[name] = ep.GlobalPosition
		}
	}
	*eventsSinceCheckpoint++
	if *eventsSinceCheckpoint < checkpointInterval {
		return
	}
	for _, name := range names {
		if err := u.checkpointStore.SaveGlobalPosition(ctx, name, checkpoints[name]); err != nil {
			u.log.Error(ctx, "failed to save checkpoint", "projection", name, "ERROR", err)
		}
	}
	u.log.Info(ctx, "checkpoints saved", "min_position", minPosition)
	*eventsSinceCheckpoint = 0
}

// minCheckpoint returns the smallest checkpoint across all projections
// — the position the next poll should read from.
func minCheckpoint(names []string, checkpoints map[string]int64) int64 {
	m := checkpoints[names[0]]
	for _, name := range names[1:] {
		if checkpoints[name] < m {
			m = checkpoints[name]
		}
	}
	return m
}

// min returns the smaller of two durations.
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
