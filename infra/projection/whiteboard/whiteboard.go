package whiteboard

// WhiteboardStore is a projection that maintains whiteboard content per thread.
//
// This projection listens to ToolCallExecutedEvent (for write_whiteboard tool) and builds
// a read model of whiteboard state. It's used by tools to read/write whiteboard
// content without coupling to the Thread domain model.
//
// Event sourced: All state is derived from events in the EventStore.
type WhiteboardStore interface {
	// Get retrieves the whiteboard content for a thread.
	// Returns empty string if thread has no whiteboard content.
	Get(threadID string) string

	// GetAll retrieves all whiteboards (threadID -> content).
	GetAll() map[string]string

	// Update applies an event to update the projection.
	// Implements eventstore.EventApplier interface.
	Update(event any) error
}
