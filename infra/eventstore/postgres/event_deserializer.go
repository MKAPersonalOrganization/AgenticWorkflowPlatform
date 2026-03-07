package postgres

import (
	es "school-with-ai/foundation/eventsourcing"
)

// DeserializeDomainEvent deserializes a domain event from JSON based on its
// type using the shared DeserializerRegistry. The registry is populated at
// application startup by each module (thread events, billing events, etc.).
func DeserializeDomainEvent(registry *es.DeserializerRegistry, eventType string, data []byte) (es.Event, error) {
	return registry.Deserialize(eventType, data)
}
