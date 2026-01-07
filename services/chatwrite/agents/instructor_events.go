package agents

// This file re-exports domain events from the domain package for backward compatibility
// New code should import from domain directly

import (
	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core/middleware"
)

// Re-export EventType and constants
type EventType = domain.EventType

const (
	EventTypeUserMessage               = domain.EventTypeUserMessage
	EventTypeWhiteboardUpdated         = "whiteboard.updated" // Now handled by middleware
	EventTypeInstructorThought         = domain.EventTypeInstructorThought
	EventTypeInstructorStrategyChanged = domain.EventTypeInstructorStrategyChanged
	EventTypeQuestionAnswered          = domain.EventTypeQuestionAnswered
	EventTypeMathQuestion              = domain.EventTypeMathQuestion
	EventTypePhysicsQuestion           = domain.EventTypePhysicsQuestion
	EventTypeChemistryQuestion         = domain.EventTypeChemistryQuestion
	EventTypeStatisticsQuestion        = domain.EventTypeStatisticsQuestion
	EventTypeLinearAlgebraQuestion     = domain.EventTypeLinearAlgebraQuestion
	EventTypeDiagrammingQuestion       = domain.EventTypeDiagrammingQuestion
	EventTypeAgentHandoff              = domain.EventTypeAgentHandoff
)

// Re-export DomainEvent type
type DomainEvent = domain.DomainEvent

// Re-export event constructors
var (
	NewWhiteboardUpdatedEvent         = middleware.NewWhiteboardUpdatedEvent
	NewInstructorThoughtEvent         = domain.NewInstructorThoughtEvent
	NewInstructorStrategyChangedEvent = domain.NewInstructorStrategyChangedEvent
	NewQuestionAnsweredEvent          = domain.NewQuestionAnsweredEvent
	NewMathQuestionEvent              = domain.NewMathQuestionEvent
	NewPhysicsQuestionEvent           = domain.NewPhysicsQuestionEvent
	NewChemistryQuestionEvent         = domain.NewChemistryQuestionEvent
	NewStatisticsQuestionEvent        = domain.NewStatisticsQuestionEvent
	NewLinearAlgebraQuestionEvent     = domain.NewLinearAlgebraQuestionEvent
	NewDiagrammingQuestionEvent       = domain.NewDiagrammingQuestionEvent
	NewAgentHandoffEvent              = domain.NewAgentHandoffEvent
	GenerateEventID                   = domain.GenerateEventID
)
