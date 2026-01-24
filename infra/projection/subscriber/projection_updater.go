// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"context"
	"errors"
	"fmt"
	"io"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"

	"github.com/jmoiron/sqlx"
	"github.com/kurrent-io/KurrentDB-Client-Go/kurrentdb"
)

// EventRouter defines the interface for dispatching events.
type EventRouter interface {
	Dispatch(ctx context.Context, event core.Event) error
}

// ProjectionUpdater subscribes to all events and updates registered projections via a router.
type ProjectionUpdater struct {
	log             *logger.Logger
	client          *kurrentdb.Client
	checkpointStore *CheckpointStore
	router          EventRouter
	shutdownChan    chan struct{}
}

// NewProjectionUpdater creates a new projection updater.
func NewProjectionUpdater(
	log *logger.Logger,
	client *kurrentdb.Client,
	db *sqlx.DB,
	router EventRouter,
) *ProjectionUpdater {
	return &ProjectionUpdater{
		log:             log,
		client:          client,
		checkpointStore: NewCheckpointStore(db),
		router:          router,
		shutdownChan:    make(chan struct{}),
	}
}

// Start begins the subscription loop. This is a blocking call.
func (u *ProjectionUpdater) Start(ctx context.Context) error {
	u.log.Info(ctx, "projection updater starting")

	// Get last checkpoint position (use "all_projections" as the checkpoint name)
	checkpoint, err := u.checkpointStore.Get(ctx, "all_projections")
	if err != nil {
		u.log.Info(ctx, "no checkpoint found, starting from beginning")
		checkpoint = CheckpointPosition{CommitPosition: 0, PreparePosition: 0}
	} else {
		u.log.Info(ctx, "resuming from checkpoint",
			"commit", checkpoint.CommitPosition,
			"prepare", checkpoint.PreparePosition)
	}

	// Subscribe to $all stream starting from checkpoint
	opts := kurrentdb.SubscribeToAllOptions{
		From: kurrentdb.Position{
			Commit:  checkpoint.CommitPosition,
			Prepare: checkpoint.PreparePosition,
		},
		Filter: kurrentdb.ExcludeSystemEventsFilter(),
	}

	stream, err := u.client.SubscribeToAll(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to subscribe to $all: %w", err)
	}
	defer func() {
		if closeErr := stream.Close(); closeErr != nil {
			u.log.Error(ctx, "failed to close subscription stream", "ERROR", closeErr)
		}
	}()

	u.log.Info(ctx, "subscribed to $all stream successfully")

	// Track events for checkpoint saving
	eventsSinceCheckpoint := 0
	const checkpointInterval = 100 // Save checkpoint every 100 events

	for {
		select {
		case <-ctx.Done():
			u.log.Info(ctx, "projection updater stopping due to context cancellation")
			return ctx.Err()

		case <-u.shutdownChan:
			u.log.Info(ctx, "projection updater stopping due to shutdown signal")
			return nil

		default:
			// Receive next message
			msg := stream.Recv()

			// Handle subscription dropped
			if msg.SubscriptionDropped != nil {
				u.log.Error(ctx, "subscription dropped",
					"reason", msg.SubscriptionDropped.Error)
				return fmt.Errorf("subscription dropped: %w", msg.SubscriptionDropped.Error)
			}

			// Handle event
			if msg.EventAppeared != nil {
				event := msg.EventAppeared.OriginalEvent()

				// Convert KurrentDB event to domain event
				domainEvent, err := u.convertEvent(event)
				if err != nil {
					// Skip unknown event types (they're not domain events we care about)
					if errors.Is(err, io.EOF) {
						continue
					}
					u.log.Warn(ctx, "failed to convert event", "eventType", event.EventType, "ERROR", err)
					continue
				}

				// Dispatch event to all registered handlers via router
				if err := u.router.Dispatch(ctx, domainEvent); err != nil {
					u.log.Error(ctx, "failed to dispatch event",
						"eventType", event.EventType,
						"ERROR", err)
				}

				eventsSinceCheckpoint++

				// TODO: Fix checkpoint saving - need to determine correct way to get position from EventAppeared
				// Save checkpoint periodically
				if eventsSinceCheckpoint >= checkpointInterval {
					// Temporarily skip checkpoint saving until we fix position access
					u.log.Info(ctx, "skipping checkpoint save (position tracking TODO)",
						"events_processed", eventsSinceCheckpoint)

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

// convertEvent converts a KurrentDB RecordedEvent to a domain event.
func (u *ProjectionUpdater) convertEvent(event *kurrentdb.RecordedEvent) (core.Event, error) {
	return deserializeDomainEvent(event.EventType, event.Data)
}
