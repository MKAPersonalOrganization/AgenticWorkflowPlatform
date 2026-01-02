package agent

import (
	"encoding/json"
	"time"
)

// EventType represents the type of domain event
type EventType string

const (
	EventTypeWhiteboardUpdated         EventType = "whiteboard.updated"
	EventTypeInstructorThought         EventType = "instructor.thought"
	EventTypeInstructorStrategyChanged EventType = "instructor.strategy_changed"
	EventTypeMathQuestion              EventType = "question.math"
	EventTypePhysicsQuestion           EventType = "question.physics"
	EventTypeChemistryQuestion         EventType = "question.chemistry"
	EventTypeStatisticsQuestion        EventType = "question.statistics"
	EventTypeQuestionAnswered          EventType = "question.answered"
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

// Event constructors

func NewWhiteboardUpdatedEvent(threadID, agentName, content, action string) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypeWhiteboardUpdated,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"agent_name": agentName,
			"content":    content,
			"action":     action,
		},
	}
}

func NewInstructorThoughtEvent(threadID, instructorName, thought string, isPrivate bool) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypeInstructorThought,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"instructor_name": instructorName,
			"thought":         thought,
			"is_private":      isPrivate,
		},
	}
}

func NewInstructorStrategyChangedEvent(threadID, instructorName, oldStrategy, newStrategy, reason string) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypeInstructorStrategyChanged,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"instructor_name": instructorName,
			"old_strategy":    oldStrategy,
			"new_strategy":    newStrategy,
			"reason":          reason,
		},
	}
}

// Subject-specific question events for agent activation

func NewMathQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypeMathQuestion,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"question":         question,
			"emitted_by":       emittedBy,
			"context":          context,
			"whiteboard_state": whiteboardState,
		},
	}
}

func NewPhysicsQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypePhysicsQuestion,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"question":         question,
			"emitted_by":       emittedBy,
			"context":          context,
			"whiteboard_state": whiteboardState,
		},
	}
}

func NewChemistryQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypeChemistryQuestion,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"question":         question,
			"emitted_by":       emittedBy,
			"context":          context,
			"whiteboard_state": whiteboardState,
		},
	}
}

func NewStatisticsQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypeStatisticsQuestion,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"question":         question,
			"emitted_by":       emittedBy,
			"context":          context,
			"whiteboard_state": whiteboardState,
		},
	}
}

func NewQuestionAnsweredEvent(threadID, subject, answer, answeredBy string, isComplete bool, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        generateEventID(),
		Type:      EventTypeQuestionAnswered,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"subject":          subject,
			"answer":           answer,
			"answered_by":      answeredBy,
			"is_complete":      isComplete,
			"whiteboard_state": whiteboardState,
		},
	}
}

// Helper function to generate unique event IDs
func generateEventID() string {
	return time.Now().Format("20060102150405.000000")
}
