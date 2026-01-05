package sdk

import (
	"context"
)

// Projection is a read model built from events.
//
// In event sourcing, projections are derived state - they are built by
// replaying events from the EventStore. Key characteristics:
//
//   - Projections are read-only from the agent's perspective
//   - Projections can be rebuilt at any time from the EventStore
//   - Projections are optimized for specific query patterns
//   - Multiple projections can exist for the same event stream
//
// The projection workflow:
//  1. Events are appended to EventStore (source of truth)
//  2. Projection.Apply() is called for each event
//  3. Agents read from projections (never directly from EventStore)
//
// Example projections:
//   - ThreadProjection: Thread state with messages and metadata
//   - UserActivityProjection: User engagement metrics
//   - SearchIndexProjection: Full-text search index
type Projection interface {
	// Apply updates the projection with a single event.
	// This is called whenever a new event is appended to the EventStore.
	Apply(ctx context.Context, event Event) error

	// Rebuild reconstructs the projection from scratch using events from the EventStore.
	// This is useful for:
	//   - Initial projection setup
	//   - Fixing corrupted projection state
	//   - Applying projection logic changes retroactively
	Rebuild(ctx context.Context, store EventStore, streamID string) error
}

// ProjectionWithSnapshot extends Projection with snapshotting capability.
// Snapshots allow faster rebuilds by storing intermediate projection state.
type ProjectionWithSnapshot interface {
	Projection

	// Snapshot returns the current projection state as bytes.
	// This can be stored and used to speed up future rebuilds.
	Snapshot() ([]byte, error)

	// RestoreFromSnapshot restores projection state from a snapshot.
	// After restoration, only events after the snapshot need to be replayed.
	RestoreFromSnapshot(data []byte) error

	// SnapshotVersion returns the event version this snapshot represents.
	SnapshotVersion() int64
}

// ThreadProjection is the projection for thread state.
//
// It provides read access to:
//   - Thread messages (conversation history)
//   - Thread metadata (title, timestamps, current agent)
//   - Agent collaboration state (whiteboard content)
//
// Agents read from this projection to understand conversation context.
// Agents write by emitting events to the EventStore.
type ThreadProjection interface {
	Projection

	// --- Read Methods (agents read from here) ---

	// GetThread retrieves the full thread state by ID.
	// Returns nil if thread doesn't exist.
	GetThread(threadID string) (*ThreadView, bool)

	// GetMessages retrieves all messages for a thread.
	GetMessages(threadID string) []Message

	// GetAgentThread retrieves the conversation state for agent processing.
	// This is the primary method agents use to get thread context.
	GetAgentThread(threadID string) (AgentThread, bool)

	// ListThreads returns all threads, sorted by most recent first.
	ListThreads() []ThreadView

	// --- Query Methods ---

	// ThreadExists checks if a thread has any events.
	ThreadExists(threadID string) bool
}

// ThreadView is the read model for a thread.
// This is what agents see when they read thread state.
type ThreadView struct {
	ID                string
	Title             string
	Messages          []Message
	CreatedAt         int64 // Unix timestamp
	UpdatedAt         int64 // Unix timestamp
	CurrentAgent      string
	WhiteboardContent string
	MessageCount      int
	Version           int64 // Event version this view is at
}

// Message represents a message in the thread view.
type Message struct {
	ID         string
	Role       string // "user", "assistant", "system", "tool"
	Content    string
	AgentName  string // Which agent produced this message
	ToolCalls  []ToolCallInfo
	ToolCallID string // For tool result messages
	Timestamp  int64  // Unix timestamp
}

// ToolCallInfo represents a tool call in a message.
type ToolCallInfo struct {
	ID        string
	Name      string
	Arguments string
}

// ProjectionManager manages multiple projections and their lifecycle.
type ProjectionManager interface {
	// Register adds a projection to be managed.
	Register(name string, projection Projection) error

	// Get retrieves a projection by name.
	Get(name string) (Projection, bool)

	// RebuildAll rebuilds all registered projections from the EventStore.
	RebuildAll(ctx context.Context, store EventStore) error
}
