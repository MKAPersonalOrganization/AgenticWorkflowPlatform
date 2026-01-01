package infra

import (
	"sync"

	"school-with-ai/services/agent"
)

// InMemoryEventLog is an in-memory implementation of EventLog
type InMemoryEventLog struct {
	mu     sync.RWMutex
	events map[string][]agent.Event
}

// NewInMemoryEventLog creates a new in-memory event log
func NewInMemoryEventLog() *InMemoryEventLog {
	return &InMemoryEventLog{
		events: make(map[string][]agent.Event),
	}
}

// Append adds one or more events to a thread's history
func (s *InMemoryEventLog) Append(threadID string, events ...agent.Event) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.events[threadID] = append(s.events[threadID], events...)
	return nil
}

// History retrieves all events for a thread in order
func (s *InMemoryEventLog) History(threadID string) ([]agent.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.events[threadID], nil
}
