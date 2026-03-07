package domain

import (
	es "school-with-ai/foundation/eventsourcing"
)

// EventStore interface defines the contract for persisting events.
type EventStore = es.EventStore

// Repository provides a generic interface for working with any aggregate that
// implements the Aggregate interface. It handles loading from events, saving
// uncommitted events, and optimistic concurrency.
type Repository[T interface {
	Aggregate
	LoadFromHistory(events []Event, apply func(Event)) error
}] = es.Repository[T]

// NewRepository creates a new generic repository for any aggregate type.
func NewRepository[T interface {
	Aggregate
	LoadFromHistory(events []Event, apply func(Event)) error
}](
	eventStore EventStore,
	newAggregate func(id string) T,
	getStreamName func(T) string,
) *Repository[T] {
	return es.NewRepository(eventStore, newAggregate, getStreamName)
}
