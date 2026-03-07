package domain

import (
	es "school-with-ai/foundation/eventsourcing"
)

// EventEnvelope wraps an event with metadata about its position in the stream.
type EventEnvelope = es.EventEnvelope

// Aggregate is the base interface for all event-sourced aggregates.
type Aggregate = es.Aggregate

// BaseAggregate provides common functionality for event-sourced aggregates.
// Embed this in your concrete aggregates.
type BaseAggregate = es.BaseAggregate

// StreamName generates a consistent stream name for an aggregate.
// Format: {aggregateType}-{aggregateID}
var StreamName = es.StreamName
