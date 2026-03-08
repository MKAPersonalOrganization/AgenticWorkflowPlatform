package eventsourcing

import (
	"context"
	"encoding/json"
)

// SnapshotStore persists aggregate snapshots to accelerate event replay.
// Snapshots are an optimisation — the system must work correctly without them.
type SnapshotStore interface {
	// LoadSnapshot returns the most recent snapshot for the given stream.
	// Returns nil, nil if no snapshot exists (not an error — just no snapshot yet).
	LoadSnapshot(ctx context.Context, streamName string) (*Snapshot, error)

	// SaveSnapshot persists a snapshot for the given stream.
	SaveSnapshot(ctx context.Context, snapshot *Snapshot) error
}

// Snapshot holds a serialised aggregate state at a specific version.
type Snapshot struct {
	StreamName string          `json:"stream_name"`
	Version    int64           `json:"version"`
	Data       json.RawMessage `json:"data"`
}

// Snapshotable aggregates can serialise/deserialise their state for snapshots.
// Aggregates that embed BaseAggregate can implement this interface to opt-in
// to snapshot support. The repository will automatically use it when a
// SnapshotStore is configured.
type Snapshotable interface {
	// MarshalSnapshot serialises the aggregate's current state.
	MarshalSnapshot() (json.RawMessage, error)

	// UnmarshalSnapshot restores the aggregate's state from a snapshot.
	// After this call, the aggregate's version should be set to the
	// snapshot version — the repository handles this.
	UnmarshalSnapshot(data json.RawMessage) error
}

// SnapshotInterval is the number of new events after which a snapshot is taken.
const SnapshotInterval int64 = 50
