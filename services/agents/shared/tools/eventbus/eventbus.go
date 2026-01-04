package eventbus

import (
	"sync"

	"school-with-ai/services/agents"
)

// EventBus manages event subscriptions and routing.
// It maintains a mapping of event types to agents that listen to them.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[agents.EventType][]string // event type -> list of agent names
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[agents.EventType][]string),
	}
}

// Subscribe registers an agent to receive events of the given types
func (bus *EventBus) Subscribe(agentName string, events []agents.EventType) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	for _, eventType := range events {
		// Check if agent is already subscribed
		existing := bus.subscribers[eventType]
		alreadySubscribed := false
		for _, name := range existing {
			if name == agentName {
				alreadySubscribed = true
				break
			}
		}
		if !alreadySubscribed {
			bus.subscribers[eventType] = append(bus.subscribers[eventType], agentName)
		}
	}
}

// GetSubscribers returns the list of agent names subscribed to an event type
func (bus *EventBus) GetSubscribers(eventType agents.EventType) []string {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	subscribers := bus.subscribers[eventType]
	if subscribers == nil {
		return []string{}
	}

	// Return a copy to avoid data races
	result := make([]string, len(subscribers))
	copy(result, subscribers)
	return result
}

// GetFirstSubscriber returns the first subscriber for an event type.
// Returns empty string if no subscribers exist.
func (bus *EventBus) GetFirstSubscriber(eventType agents.EventType) string {
	subscribers := bus.GetSubscribers(eventType)
	if len(subscribers) > 0 {
		return subscribers[0]
	}
	return ""
}

// HasSubscribers returns true if any agent is subscribed to the event type
func (bus *EventBus) HasSubscribers(eventType agents.EventType) bool {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	return len(bus.subscribers[eventType]) > 0
}

// AllSubscriptions returns all subscriptions (for debugging)
func (bus *EventBus) AllSubscriptions() map[agents.EventType][]string {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	result := make(map[agents.EventType][]string)
	for k, v := range bus.subscribers {
		result[k] = append([]string{}, v...)
	}
	return result
}
