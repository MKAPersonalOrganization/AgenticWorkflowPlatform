// Package subscriber provides event subscription for updating projections.
package subscriber

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"

	"github.com/jmoiron/sqlx"
	"github.com/kurrent-io/KurrentDB-Client-Go/kurrentdb"
)

// Projection defines the interface that projections must implement to receive events.
type Projection interface {
	Apply(ctx context.Context, event core.Event) error
	Name() string
}

// ProjectionUpdater subscribes to all events and updates registered projections.
type ProjectionUpdater struct {
	log             *logger.Logger
	client          *kurrentdb.Client
	checkpointStore *CheckpointStore
	projections     []Projection
	shutdownChan    chan struct{}
}

// NewProjectionUpdater creates a new projection updater.
func NewProjectionUpdater(
	log *logger.Logger,
	client *kurrentdb.Client,
	db *sqlx.DB,
	projections ...Projection,
) *ProjectionUpdater {
	return &ProjectionUpdater{
		log:             log,
		client:          client,
		checkpointStore: NewCheckpointStore(db),
		projections:     projections,
		shutdownChan:    make(chan struct{}),
	}
}

// Start begins the subscription loop. This is a blocking call.
func (u *ProjectionUpdater) Start(ctx context.Context) error {
	u.log.Info(ctx, "projection updater starting", "projections", len(u.projections))

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
	defer stream.Close()

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

				// Apply event to all projections
				for _, projection := range u.projections {
					if err := projection.Apply(ctx, domainEvent); err != nil {
						u.log.Error(ctx, "failed to apply event to projection",
							"projection", projection.Name(),
							"eventType", event.EventType,
							"ERROR", err)
						// Continue to other projections even if one fails
					}
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
	// Deserialize based on event type
	switch event.EventType {
	case "ThreadCreated":
		var evt domain.ThreadCreated
		if err := json.Unmarshal(event.Data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadCreated: %w", err)
		}
		return evt, nil

	case "MessageAdded":
		var evt domain.MessageAdded
		if err := json.Unmarshal(event.Data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal MessageAdded: %w", err)
		}
		return evt, nil

	case "ThreadTitleChanged":
		var evt domain.ThreadTitleChanged
		if err := json.Unmarshal(event.Data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadTitleChanged: %w", err)
		}
		return evt, nil

	case "ThreadDeleted":
		var evt domain.ThreadDeleted
		if err := json.Unmarshal(event.Data, &evt); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ThreadDeleted: %w", err)
		}
		return evt, nil

	default:
		// Skip unknown event types (not domain events we care about)
		return nil, fmt.Errorf("unknown event type: %s", event.EventType)
	}
}
