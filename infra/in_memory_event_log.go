package infra

import (
	"sync"

	"school-with-ai/services/sdk"
)

// InMemoryEventLog is an in-memory implementation of EventLog
type InMemoryEventLog struct {
	mu     sync.RWMutex
	events map[string][]sdk.Event
}

// NewInMemoryEventLog creates a new in-memory event log
func NewInMemoryEventLog() *InMemoryEventLog {
	return &InMemoryEventLog{
		events: make(map[string][]sdk.Event),
	}
}

// Append adds one or more events to a thread's history
func (s *InMemoryEventLog) Append(threadID string, events ...sdk.Event) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events[threadID] = append(s.events[threadID], events...)
	return nil
}

// History retrieves all events for a thread in order
func (s *InMemoryEventLog) History(threadID string) ([]sdk.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.events[threadID], nil
}
