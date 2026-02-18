// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"
	pgstore "school-with-ai/infra/eventstore/postgres"

	"school-with-ai/foundation/logger"

	"github.com/jmoiron/sqlx"
)

// EventRouter defines the interface for dispatching events.
type EventRouter interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

// ProjectionUpdater polls the Postgres event store for new events and dispatches
// them to registered projections via a router using a pull-based polling loop.
type ProjectionUpdater struct {
	log             *logger.Logger
	eventStore      *pgstore.PostgresEventStore
	checkpointStore *CheckpointStore
	router          EventRouter
	shutdownChan    chan struct{}

	// Polling configuration
	pollInterval time.Duration
	batchSize    int
	idleBackoff  time.Duration
	maxBackoff   time.Duration
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

// NewProjectionUpdater creates a new projection updater that polls Postgres.
func NewProjectionUpdater(
	log *logger.Logger,
	eventStore *pgstore.PostgresEventStore,
	db *sqlx.DB,
	router EventRouter,
	opts ...ProjectionUpdaterOption,
) *ProjectionUpdater {
	u := &ProjectionUpdater{
		log:             log,
		eventStore:      eventStore,
		checkpointStore: NewCheckpointStore(db),
		router:          router,
		shutdownChan:    make(chan struct{}),
		pollInterval:    100 * time.Millisecond,
		batchSize:       500,
		idleBackoff:     500 * time.Millisecond,
		maxBackoff:      5 * time.Second,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// Start begins the polling loop. This is a blocking call.
func (u *ProjectionUpdater) Start(ctx context.Context) error {
	u.log.Info(ctx, "projection updater starting (postgres polling mode)")

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

	for {
		select {
		case <-ctx.Done():
			// Save final checkpoint before exit
			if eventsSinceCheckpoint > 0 {
				if saveErr := u.checkpointStore.SaveGlobalPosition(ctx, "all_projections", lastPosition); saveErr != nil {
					u.log.Error(ctx, "failed to save final checkpoint", "ERROR", saveErr)
				}
			}
			u.log.Info(ctx, "projection updater stopping due to context cancellation")
			return ctx.Err()

		case <-u.shutdownChan:
			if eventsSinceCheckpoint > 0 {
				if saveErr := u.checkpointStore.SaveGlobalPosition(ctx, "all_projections", lastPosition); saveErr != nil {
					u.log.Error(ctx, "failed to save final checkpoint", "ERROR", saveErr)
				}
			}
			u.log.Info(ctx, "projection updater stopping due to shutdown signal")
			return nil

		default:
			// Poll for new events
			events, err := u.eventStore.ReadAllFromPosition(ctx, lastPosition, u.batchSize)
			if err != nil {
				u.log.Error(ctx, "failed to poll events", "ERROR", err)
				time.Sleep(backoff)
				backoff = min(backoff*2, u.maxBackoff)
				continue
			}

			if len(events) == 0 {
				// No new events — idle backoff
				time.Sleep(backoff)
				backoff = min(backoff*2, u.idleBackoff)
				continue
			}

			// Reset backoff on successful read
			backoff = u.pollInterval

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

// Shutdown signals the updater to stop gracefully.
func (u *ProjectionUpdater) Shutdown() {
	close(u.shutdownChan)
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
