package handlers

// EventEmitter is an interface for emitting domain events
type EventEmitter interface {
	// Emit sends an event to the underlying transport
	Emit(event any) error
}
