package event

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	es "school-with-ai/foundation/eventsourcing"
)

// HandlerFunc is a function that handles a specific event type.
type HandlerFunc func(ctx context.Context, event es.Event) error

// Router dispatches events to registered handlers based on their type.
type Router struct {
	mu          sync.RWMutex
	handlers    map[reflect.Type][]HandlerFunc
	anyHandlers []HandlerFunc
}

// NewRouter creates a new event router.
func NewRouter() *Router {
	return &Router{
		handlers: make(map[reflect.Type][]HandlerFunc),
	}
}

// Register registers a handler for a specific event type.
// The eventPrototype should be a pointer to the event struct (e.g., &domain.ThreadCreated{}).
func (r *Router) Register(eventPrototype any, handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t := reflect.TypeOf(eventPrototype)
	r.handlers[t] = append(r.handlers[t], handler)
}

// RegisterAny registers a handler that receives all events.
func (r *Router) RegisterAny(handler HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.anyHandlers = append(r.anyHandlers, handler)
}

// Dispatch sends the event to all registered handlers for its type.
// Handlers are isolated: a failure in one handler does not prevent others from running.
// The first error encountered is returned, but all handlers are always invoked.
func (r *Router) Dispatch(ctx context.Context, event es.Event) error {
	r.mu.RLock()
	t := reflect.TypeOf(event)
	// Copy slice headers to prevent data races if Register is called concurrently.
	anyHandlers := make([]HandlerFunc, len(r.anyHandlers))
	copy(anyHandlers, r.anyHandlers)
	handlers := make([]HandlerFunc, len(r.handlers[t]))
	copy(handlers, r.handlers[t])
	r.mu.RUnlock()

	var firstErr error

	for _, handler := range anyHandlers {
		if err := handler(ctx, event); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("any handler failed: %w", err)
		}
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("event handler failed for %T: %w", event, err)
		}
	}

	return firstErr
}
