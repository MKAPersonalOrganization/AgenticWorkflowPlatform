package domain

import (
	"fmt"
	"school-with-ai/sdk/agentsdk/core"
	"time"
)

// Event is an alias for core.Event for convenience.
type Event = core.Event

// EventEnvelope wraps an event with metadata about its position in the stream.
type EventEnvelope struct {
	Data           core.Event
	EventID        string
	StreamPosition int64
	LogPosition    int64
	Timestamp      time.Time
}

// AggregateState holds the common state for event-sourced aggregates.
// Embed this in your concrete aggregate types to get event sourcing capabilities.
//
// Example:
//
//	type Thread struct {
//	    AggregateState
//	    Title    string
//	    Messages []Message
//	}
type AggregateState struct {
	ID                string
	Version           int64
	uncommittedEvents []core.Event
}

// NewAggregateState creates a new aggregate state with the given ID.
func NewAggregateState(id string) AggregateState {
	return AggregateState{
		ID:                id,
		Version:           0,
		uncommittedEvents: make([]core.Event, 0),
	}
}

// Enqueue adds a new event to the uncommitted events queue.
// Call this after validating business rules and before applying the event.
func (a *AggregateState) Enqueue(event core.Event) {
	a.uncommittedEvents = append(a.uncommittedEvents, event)
}

// DequeueUncommittedEvents returns all uncommitted events and clears the queue.
// This is typically called by the repository after persisting events to the store.
func (a *AggregateState) DequeueUncommittedEvents() []core.Event {
	events := a.uncommittedEvents
	a.uncommittedEvents = make([]core.Event, 0)
	return events
}

// UncommittedEventsCount returns the number of uncommitted events.
func (a *AggregateState) UncommittedEventsCount() int {
	return len(a.uncommittedEvents)
}

// IncrementVersion increments the version counter.
// Called after successfully applying an event.
func (a *AggregateState) IncrementVersion() {
	a.Version++
}

// LoadFromHistory rebuilds aggregate state by replaying events.
// Pass your concrete aggregate's Evolve function to apply state changes.
//
// Example:
//
//	err := thread.LoadFromHistory(events, thread.Evolve)
func (a *AggregateState) LoadFromHistory(events []core.Event, evolve func(core.Event)) error {
	for i, event := range events {
		// Validate that event belongs to this aggregate
		aggID := event.AggregateID()
		if aggID != a.ID {
			return fmt.Errorf("event %d (type=%s) aggregate ID %q does not match %s",
				i, event.EventType(), aggID, a.ID)
		}

		// Apply the event to update state
		evolve(event)

		// Increment version to track number of applied events
		a.IncrementVersion()
	}
	return nil
}

// StreamName generates a consistent stream name for an aggregate.
// Format: {aggregateType}-{aggregateID}
// Example: "Thread-123e4567-e89b-12d3-a456-426614174000"
func StreamName(aggregateType, aggregateID string) string {
	return fmt.Sprintf("%s-%s", aggregateType, aggregateID)
}
