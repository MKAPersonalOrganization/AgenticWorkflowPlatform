package taskmanagement

import (
	"school-with-ai/foundation/agentsdk/core"
)

// DomainEventNotifier emits domain events without any UI conversion.
// This keeps the agent layer decoupled from presentation concerns.
type DomainEventNotifier struct {
	emitter EventEmitter
}

// NewDomainEventNotifier creates a new domain event notifier.
func NewDomainEventNotifier(emitter EventEmitter) *DomainEventNotifier {
	return &DomainEventNotifier{
		emitter: emitter,
	}
}

// HandleDomainEvent emits domain events directly without conversion.
func (n *DomainEventNotifier) HandleDomainEvent(event core.Event) error {
	return n.emitter.Emit(event)
}
