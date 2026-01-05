package sdk

import (
	"context"
	"sync"
)

// InMemoryEventStore is an in-memory implementation of EventStore.
//
// This implementation is suitable for:
//   - Development and testing
//   - Single-process deployments
//   - Scenarios where event persistence across restarts isn't required
//
// For production deployments requiring persistence, implement EventStore
// with a database backend (PostgreSQL, EventStoreDB, etc.).
type InMemoryEventStore struct {
	mu          sync.RWMutex
	streams     map[string][]Event
	subscribers map[string][]eventSubscriber
	allSubs     []eventSubscriber
	subCounter  int
}

type eventSubscriber struct {
	id      int
	handler EventHandler
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		streams:     make(map[string][]Event),
		subscribers: make(map[string][]eventSubscriber),
		allSubs:     make([]eventSubscriber, 0),
	}
}

// Append adds events to a stream.
func (s *InMemoryEventStore) Append(ctx context.Context, streamID string, events ...Event) (int64, error) {
	if len(events) == 0 {
		version, _ := s.StreamVersion(ctx, streamID)
		return version, nil
	}

	s.mu.Lock()
	s.streams[streamID] = append(s.streams[streamID], events...)
	newVersion := int64(len(s.streams[streamID]))

	// Get subscribers while holding lock
	streamSubs := s.subscribers[streamID]
	allSubs := s.allSubs
	s.mu.Unlock()

	// Notify subscribers (outside lock to prevent deadlocks)
	for _, event := range events {
		for _, sub := range streamSubs {
			sub.handler(ctx, streamID, event)
		}
		for _, sub := range allSubs {
			sub.handler(ctx, streamID, event)
		}
	}

	return newVersion, nil
}

// AppendWithExpectedVersion appends events only if the stream is at the expected version.
func (s *InMemoryEventStore) AppendWithExpectedVersion(ctx context.Context, streamID string, expectedVersion int64, events ...Event) (int64, error) {
	s.mu.Lock()

	currentVersion := int64(len(s.streams[streamID]))

	// Special case: expectedVersion = -1 means expect stream doesn't exist
	if expectedVersion == -1 {
		if currentVersion > 0 {
			s.mu.Unlock()
			return currentVersion, ErrConcurrencyConflict
		}
	} else if currentVersion != expectedVersion {
		s.mu.Unlock()
		return currentVersion, ErrConcurrencyConflict
	}

	s.streams[streamID] = append(s.streams[streamID], events...)
	newVersion := int64(len(s.streams[streamID]))

	// Get subscribers while holding lock
	streamSubs := s.subscribers[streamID]
	allSubs := s.allSubs
	s.mu.Unlock()

	// Notify subscribers
	for _, event := range events {
		for _, sub := range streamSubs {
			sub.handler(ctx, streamID, event)
		}
		for _, sub := range allSubs {
			sub.handler(ctx, streamID, event)
		}
	}

	return newVersion, nil
}

// ReadStream reads all events for a stream from a specific version.
func (s *InMemoryEventStore) ReadStream(ctx context.Context, streamID string, fromVersion int64) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.streams[streamID]
	if fromVersion >= int64(len(events)) {
		return []Event{}, nil
	}

	return events[fromVersion:], nil
}

// ReadAll reads all events for a stream.
func (s *InMemoryEventStore) ReadAll(ctx context.Context, streamID string) ([]Event, error) {
	return s.ReadStream(ctx, streamID, 0)
}

// StreamVersion returns the current version for a stream.
func (s *InMemoryEventStore) StreamVersion(ctx context.Context, streamID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return int64(len(s.streams[streamID])), nil
}

// StreamExists checks if a stream has any events.
func (s *InMemoryEventStore) StreamExists(ctx context.Context, streamID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, exists := s.streams[streamID]
	return exists && len(events) > 0, nil
}

// ListStreams returns all stream IDs.
func (s *InMemoryEventStore) ListStreams(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.streams))
	for id, events := range s.streams {
		if len(events) > 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// DeleteStream removes all events for a stream.
func (s *InMemoryEventStore) DeleteStream(ctx context.Context, streamID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.streams, streamID)
	delete(s.subscribers, streamID)
	return nil
}

// Subscribe creates a subscription to events for a specific stream.
func (s *InMemoryEventStore) Subscribe(ctx context.Context, streamID string, handler EventHandler) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subCounter++
	sub := eventSubscriber{
		id:      s.subCounter,
		handler: handler,
	}

	s.subscribers[streamID] = append(s.subscribers[streamID], sub)

	// Return unsubscribe function
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		subs := s.subscribers[streamID]
		for i, existing := range subs {
			if existing.id == sub.id {
				s.subscribers[streamID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}, nil
}

// SubscribeAll creates a subscription to all events.
func (s *InMemoryEventStore) SubscribeAll(ctx context.Context, handler EventHandler) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subCounter++
	sub := eventSubscriber{
		id:      s.subCounter,
		handler: handler,
	}

	s.allSubs = append(s.allSubs, sub)

	// Return unsubscribe function
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		for i, existing := range s.allSubs {
			if existing.id == sub.id {
				s.allSubs = append(s.allSubs[:i], s.allSubs[i+1:]...)
				break
			}
		}
	}, nil
}

// --- Helper methods for testing and development ---

// Clear removes all events from all streams.
func (s *InMemoryEventStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.streams = make(map[string][]Event)
}

// StreamCount returns the number of streams.
func (s *InMemoryEventStore) StreamCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, events := range s.streams {
		if len(events) > 0 {
			count++
		}
	}
	return count
}

// EventCount returns the total number of events across all streams.
func (s *InMemoryEventStore) EventCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, events := range s.streams {
		count += len(events)
	}
	return count
}

// Verify InMemoryEventStore implements EventStoreWithSubscription and EventStoreWithOptimisticLocking
var _ EventStoreWithSubscription = (*InMemoryEventStore)(nil)
var _ EventStoreWithOptimisticLocking = (*InMemoryEventStore)(nil)
