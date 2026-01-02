package sdk

// EventLog is the append-only log of everything that happened in a thread
type EventLog interface {
	// Append adds one or more events to a thread's history
	Append(threadID string, events ...Event) error

	// History retrieves all events for a thread in order
	History(threadID string) ([]Event, error)
}
