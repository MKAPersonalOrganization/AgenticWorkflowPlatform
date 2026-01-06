package domain

import (
	"encoding/json"
	"time"
)

// EventType represents the type of domain event
type EventType string

// Common instructor event types
const (
	EventTypeUserMessage               EventType = "user.message"
	EventTypeWhiteboardUpdated         EventType = "whiteboard.updated"
	EventTypeInstructorThought         EventType = "instructor.thought"
	EventTypeInstructorStrategyChanged EventType = "instructor.strategy_changed"
	EventTypeQuestionAnswered          EventType = "question.answered"
	EventTypeMathQuestion              EventType = "question.math"
	EventTypePhysicsQuestion           EventType = "question.physics"
	EventTypeChemistryQuestion         EventType = "question.chemistry"
	EventTypeStatisticsQuestion        EventType = "question.statistics"
	EventTypeLinearAlgebraQuestion     EventType = "question.linearalgebra"
	EventTypeDiagrammingQuestion       EventType = "question.diagramming"
	EventTypeAgentHandoff              EventType = "agent.handoff"
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

func NewWhiteboardUpdatedEvent(threadID, agentName, content, action string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
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
		ID:        GenerateEventID(),
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
		ID:        GenerateEventID(),
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

func NewQuestionAnsweredEvent(threadID, subject, answer, answeredBy, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
		Type:      EventTypeQuestionAnswered,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"subject":          subject,
			"answer":           answer,
			"answered_by":      answeredBy,
			"whiteboard_state": whiteboardState,
		},
	}
}

// NewMathQuestionEvent creates a math question event
func NewMathQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
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

// NewPhysicsQuestionEvent creates a physics question event
func NewPhysicsQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
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

// NewChemistryQuestionEvent creates a chemistry question event
func NewChemistryQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
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

// NewStatisticsQuestionEvent creates a statistics question event
func NewStatisticsQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
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

// NewLinearAlgebraQuestionEvent creates a linear algebra question event
func NewLinearAlgebraQuestionEvent(threadID, question, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
		Type:      EventTypeLinearAlgebraQuestion,
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

// NewDiagrammingQuestionEvent creates a diagramming question event
func NewDiagrammingQuestionEvent(threadID, request, emittedBy, context, whiteboardState string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
		Type:      EventTypeDiagrammingQuestion,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"request":          request,
			"emitted_by":       emittedBy,
			"context":          context,
			"whiteboard_state": whiteboardState,
		},
	}
}

// NewAgentHandoffEvent creates an agent handoff event
func NewAgentHandoffEvent(threadID, fromAgent, toAgent, reason string) *DomainEvent {
	return &DomainEvent{
		ID:        GenerateEventID(),
		Type:      EventTypeAgentHandoff,
		Timestamp: time.Now(),
		ThreadID:  threadID,
		Version:   1,
		Data: map[string]interface{}{
			"from_agent": fromAgent,
			"to_agent":   toAgent,
			"reason":     reason,
		},
	}
}

// GenerateEventID generates unique event IDs (exported for use by agent packages)
func GenerateEventID() string {
	return time.Now().Format("20060102150405.000000")
}
