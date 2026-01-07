package domain

import (
	"encoding/json"
	"time"
)

// EventType represents the type of domain event
type EventType string

// Common instructor event types
const (
	EventTypeUserMessage EventType = "user.message"
)

// DomainEvent is the unified event structure for all system events
type DomainEvent struct {
	// Core event metadata
	ID        string    `json:"id"`
	Type      EventType `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	ThreadID  string    `json:"thread_id"`
	Version   int       `json:"version"` // For event versioning/schema evolution

	// Event payload - flexible structure for different event types
	Data map[string]interface{} `json:"data"`

	// Additional metadata
	Metadata map[string]string `json:"metadata,omitempty"`

	// Correlation for event chains
	CorrelationID string `json:"correlation_id,omitempty"`
	CausationID   string `json:"causation_id,omitempty"` // ID of event that caused this one
}

// Implement Event interface
func (e *DomainEvent) EventType() string         { return string(e.Type) }
func (e *DomainEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e *DomainEvent) AggregateID() string       { return e.ThreadID }

// Helper methods to extract typed data from the event

func (e *DomainEvent) GetString(key string) string {
	if val, ok := e.Data[key].(string); ok {
		return val
	}
	return ""
}

func (e *DomainEvent) GetBool(key string) bool {
	if val, ok := e.Data[key].(bool); ok {
		return val
	}
	return false
}

func (e *DomainEvent) GetInt(key string) int {
	if val, ok := e.Data[key].(float64); ok { // JSON numbers are float64
		return int(val)
	}
	return 0
}

func (e *DomainEvent) MarshalPayload() ([]byte, error) {
	return json.Marshal(e.Data)
}

// Common instructor event constructors

// GenerateEventID generates unique event IDs (exported for use by agent packages)
func GenerateEventID() string {
	return time.Now().Format("20060102150405.000000")
}
