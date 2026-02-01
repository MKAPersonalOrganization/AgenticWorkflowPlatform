package event

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"school-with-ai/domain"
)

// HandlerFunc is a function that handles a specific event type.
type HandlerFunc func(ctx context.Context, event domain.Event) error

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
func (r *Router) Dispatch(ctx context.Context, event domain.Event) error {
	r.mu.RLock()
	t := reflect.TypeOf(event)
	handlers := r.handlers[t]
	anyHandlers := r.anyHandlers
	r.mu.RUnlock()

	for _, handler := range anyHandlers {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("any handler failed: %w", err)
		}
	}

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("event handler failed for %T: %w", event, err)
		}
	}

	return nil
}
