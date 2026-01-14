package taskmanagement

// AguiEventEmitter wraps an EventEmitter and converts domain events to AGUI events.
// This adapter allows the orchestration service to remain transport-agnostic while
// the AGUI layer (HTTP/SSE) receives AGUI-specific events.
type AguiEventEmitter struct {
	underlying EventEmitter
}

// NewAguiEventEmitter creates a new AGUI event emitter adapter.
func NewAguiEventEmitter(emitter EventEmitter) *AguiEventEmitter {
	return &AguiEventEmitter{
		underlying: emitter,
	}
}

// Emit converts domain events to AGUI events and emits them.
func (a *AguiEventEmitter) Emit(event any) error {
	aguiEvents := ToAguiEvents(event)
	if aguiEvents == nil {
		// Pass through non-domain events (e.g., AGUI events that were already converted)
		return a.underlying.Emit(event)
	}

	for _, aguiEvent := range aguiEvents {
		if err := a.underlying.Emit(aguiEvent); err != nil {
			return err
		}
	}

	return nil
}
