package subscriber

import (
	es "school-with-ai/foundation/eventsourcing"
)

// deserializeDomainEvent deserializes a domain event from JSON based on its
// type using the shared DeserializerRegistry.
func deserializeDomainEvent(registry *es.DeserializerRegistry, eventType string, data []byte) (es.Event, error) {
	return registry.Deserialize(eventType, data)
}
