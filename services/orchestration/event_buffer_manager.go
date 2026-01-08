package orchestration

import (
	"sync"
)

// EventBufferManager manages event buffers for jobs (extracted from WorkerPool)
type EventBufferManager struct {
	mu           sync.RWMutex
	eventBuffers map[string]*EventBuffer
}

// NewEventBufferManager creates a new event buffer manager
func NewEventBufferManager() *EventBufferManager {
	return &EventBufferManager{
		eventBuffers: make(map[string]*EventBuffer),
	}
}

// CreateEmitter creates a new event emitter for a job
func (m *EventBufferManager) CreateEmitter(jobID string, threadID string) EventEmitter {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a buffered event storage
	buffer := NewEventBuffer(1000, threadID)
	m.eventBuffers[jobID] = buffer

	return &BufferedEventEmitter{
		jobID:  jobID,
		buffer: buffer,
	}
}

// GetEventBuffer gets the event buffer for a job
func (m *EventBufferManager) GetEventBuffer(jobID string) (*EventBuffer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buffer, exists := m.eventBuffers[jobID]
	return buffer, exists
}

// CloseEmitter closes and removes the event buffer for a job
func (m *EventBufferManager) CloseEmitter(jobID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if buffer, exists := m.eventBuffers[jobID]; exists {
		buffer.Close()
		delete(m.eventBuffers, jobID)
	}
}

// EventBuffer stores events for a job and allows multiple consumers
type EventBuffer struct {
	mu       sync.RWMutex
	events   []any
	closed   bool
	maxSize  int
	ThreadID string // Associated thread ID for this job
}

// NewEventBuffer creates a new event buffer
func NewEventBuffer(maxSize int, threadID string) *EventBuffer {
	return &EventBuffer{
		events:   make([]any, 0, maxSize),
		maxSize:  maxSize,
		ThreadID: threadID,
	}
}

// Append adds an event to the buffer
func (eb *EventBuffer) Append(event any) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.closed {
		return
	}

	eb.events = append(eb.events, event)

	// Trim if exceeds max size (keep most recent events)
	if len(eb.events) > eb.maxSize {
		eb.events = eb.events[len(eb.events)-eb.maxSize:]
	}
}

// GetEvents returns all events since the given index
func (eb *EventBuffer) GetEvents(fromIndex int) []any {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if fromIndex >= len(eb.events) {
		return nil
	}

	return eb.events[fromIndex:]
}

// Len returns the number of events
func (eb *EventBuffer) Len() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return len(eb.events)
}

// Close marks the buffer as closed
func (eb *EventBuffer) Close() {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.closed = true
}

// IsClosed returns whether the buffer is closed
func (eb *EventBuffer) IsClosed() bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.closed
}

// BufferedEventEmitter implements EventEmitter using EventBuffer
type BufferedEventEmitter struct {
	jobID  string
	buffer *EventBuffer
}

// Emit appends an event to the buffer
func (e *BufferedEventEmitter) Emit(event any) error {
	e.buffer.Append(event)
	return nil
}
