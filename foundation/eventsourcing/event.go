// Package eventsourcing provides the core building blocks for event-sourced
// aggregates: the open Event interface, BaseAggregate, generic Repository,
// EventStore, and a pluggable DeserializerRegistry.
//
// Introduced in Phase 2 of the modular-monolith migration
// (see arch/adr/012-modular-monolith-migration.md). It replaces the sealed
// domain.Event interface, allowing each module to define its own events.
//
// The domain/ package retains type aliases so existing code continues to
// compile without import changes.
package eventsourcing

import (
"fmt"
"time"
)

// Event is the base interface for all domain events.
// Unlike the former domain.Event, this interface is open — any package can
// implement it, enabling modules to define events locally.
type Event interface {
EventType() string
EventTimestamp() time.Time
AggregateID() string
}

// EventEnvelope wraps an event with metadata about its position in the stream.
type EventEnvelope struct {
Data           Event
EventID        string
StreamPosition int64
LogPosition    int64
Timestamp      time.Time
}

// StreamName generates a consistent stream name for an aggregate.
// Format: {aggregateType}-{aggregateID}
// Example: "Thread-123e4567-e89b-12d3-a456-426614174000"
func StreamName(aggregateType, aggregateID string) string {
return fmt.Sprintf("%s-%s", aggregateType, aggregateID)
}
