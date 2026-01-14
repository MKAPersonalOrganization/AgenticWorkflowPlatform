package taskmanagement

import (
	"fmt"

	"school-with-ai/foundation/agentsdk/core"
)

// UINotifier listens to domain events and notifies the UI
// This separates business logic (domain events) from presentation logic (UI events).
type UINotifier struct {
	emitter EventEmitter
}

// NewUINotifier creates a new UI notifier.
func NewUINotifier(emitter EventEmitter) *UINotifier {
	return &UINotifier{
		emitter: emitter,
	}
}

// HandleDomainEvent processes a domain event and emits corresponding UI events.
func (n *UINotifier) HandleDomainEvent(event core.Event) error {
	aguiEvents := ToAguiEvents(event)
	if aguiEvents == nil {
		return nil
	}

	for _, aguiEvent := range aguiEvents {
		if err := n.emitter.Emit(aguiEvent); err != nil {
			return fmt.Errorf("failed to emit AGUI event: %w", err)
		}
	}

	return nil
}
