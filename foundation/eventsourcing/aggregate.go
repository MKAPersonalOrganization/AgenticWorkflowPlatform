package eventsourcing

import "fmt"

// Aggregate is the base interface for all event-sourced aggregates.
type Aggregate interface {
// GetID returns the aggregate's unique identifier.
GetID() string

// GetVersion returns the current version (number of applied events).
GetVersion() int64

// DequeueUncommittedEvents returns all uncommitted events and clears the queue.
DequeueUncommittedEvents() []Event

// Apply applies an event to update the aggregate's state.
// Each aggregate must implement this method for its specific events.
Apply(event Event)
}

// BaseAggregate provides common functionality for event-sourced aggregates.
// Embed this in your concrete aggregates.
type BaseAggregate struct {
ID                string
Version           int64
uncommittedEvents []Event
}

// GetID returns the aggregate ID.
func (a *BaseAggregate) GetID() string {
return a.ID
}

// GetVersion returns the aggregate version.
func (a *BaseAggregate) GetVersion() int64 {
return a.Version
}

// Enqueue adds an event to the uncommitted events queue AND applies it to state.
// This follows the pattern from EventSourcing.NetCore where Enqueue both queues and applies.
// Version is NOT incremented here — it only tracks persisted events and is incremented in LoadFromHistory.
func (a *BaseAggregate) Enqueue(event Event, apply func(Event)) {
a.uncommittedEvents = append(a.uncommittedEvents, event)
apply(event)
}

// DequeueUncommittedEvents returns all uncommitted events and clears the queue.
func (a *BaseAggregate) DequeueUncommittedEvents() []Event {
events := a.uncommittedEvents
a.uncommittedEvents = make([]Event, 0)
return events
}

// LoadFromHistory rebuilds aggregate state by replaying events.
func (a *BaseAggregate) LoadFromHistory(events []Event, apply func(Event)) error {
for i, event := range events {
aggID := event.AggregateID()
if aggID != a.ID {
return fmt.Errorf("event %d (type=%s) aggregate ID %q does not match %s",
i, event.EventType(), aggID, a.ID)
}

apply(event)
a.Version++
}
return nil
}
