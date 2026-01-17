package subscriber

import (
	"context"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"sync"
)

// UIBroadcaster handles real-time event broadcasting to UI subscribers.
type UIBroadcaster struct {
	log         *logger.Logger
	mu          sync.RWMutex
	subscribers map[string][]chan any
}

// NewUIBroadcaster creates a new UI broadcaster.
func NewUIBroadcaster(log *logger.Logger) *UIBroadcaster {
	return &UIBroadcaster{
		log:         log,
		subscribers: make(map[string][]chan any),
	}
}

// Subscribe adds a subscriber for a specific thread.
func (b *UIBroadcaster) Subscribe(threadID string) (<-chan any, func()) {
	ch := make(chan any, 100)

	b.mu.Lock()
	b.subscribers[threadID] = append(b.subscribers[threadID], ch)
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		subs, ok := b.subscribers[threadID]
		if !ok {
			return
		}
		for i, sub := range subs {
			if sub == ch {
				b.subscribers[threadID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}
}

// Broadcast sends an event to all subscribers of the event's thread.
func (b *UIBroadcaster) Broadcast(ctx context.Context, event core.Event) error {
	threadID := event.AggregateID()
	if threadID == "" {
		return nil
	}

	b.mu.RLock()
	subs := b.subscribers[threadID]
	b.mu.RUnlock()

	if len(subs) == 0 {
		return nil
	}

	b.log.Debug(ctx, "UIBroadcaster dispatching event", "threadID", threadID, "eventType", event.EventType())

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			b.log.Warn(ctx, "UI broadcaster channel full, dropping event", "threadID", threadID)
		}
	}

	return nil
}
