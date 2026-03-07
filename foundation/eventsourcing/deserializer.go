package eventsourcing

import (
	"encoding/json"
	"fmt"
	"sync"
)

// EventFactory creates a zero-value pointer for a concrete event type.
// The returned value is then populated via json.Unmarshal.
type EventFactory func() Event

// EventTransform is an optional post-deserialize function that can mutate
// the event after JSON unmarshalling. This is used for legacy event aliases
// that need field fixups (e.g. setting IsError=true for "ToolError" events).
type EventTransform func(Event) Event

type registration struct {
	factory   EventFactory
	transform EventTransform // nil means no transform
}

// DeserializerRegistry maps event type strings to factory functions.
// Modules register their events at startup; the event store and projection
// subscriber use the registry to deserialize events from JSON.
//
// The registry is safe for concurrent use.
type DeserializerRegistry struct {
	mu            sync.RWMutex
	registrations map[string]registration
}

// NewDeserializerRegistry creates an empty registry.
func NewDeserializerRegistry() *DeserializerRegistry {
	return &DeserializerRegistry{
		registrations: make(map[string]registration),
	}
}

// Register associates an event type string with a factory.
// If an alias is needed (e.g. legacy type names), call Register for each alias
// with the same factory.
func (r *DeserializerRegistry) Register(eventType string, factory EventFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrations[eventType] = registration{factory: factory}
}

// RegisterWithTransform associates an event type string with a factory and
// a post-deserialize transform. The transform runs after json.Unmarshal and
// can mutate the event (e.g. setting flags for legacy event aliases).
func (r *DeserializerRegistry) RegisterWithTransform(eventType string, factory EventFactory, transform EventTransform) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrations[eventType] = registration{factory: factory, transform: transform}
}

// Deserialize unmarshals JSON data into the concrete event type.
// Returns an error if the event type is unknown or unmarshalling fails.
func (r *DeserializerRegistry) Deserialize(eventType string, data []byte) (Event, error) {
	r.mu.RLock()
	reg, ok := r.registrations[eventType]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}

	evt := reg.factory()
	if err := json.Unmarshal(data, evt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", eventType, err)
	}

	if reg.transform != nil {
		evt = reg.transform(evt)
	}

	return evt, nil
}
