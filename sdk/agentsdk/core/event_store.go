package core

import "context"

// EventStore is used internally by SDK for event persistence.
type EventStore interface {
	Append(ctx context.Context, streamID string, events ...Event) (int64, error)
	ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]Event, error)
	DeleteStream(ctx context.Context, streamID string) error
}
