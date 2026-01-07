package agents

// This file re-exports domain events from the domain package for backward compatibility
// New code should import from domain directly

import (
	"school-with-ai/domain"
)

// Re-export EventType and constants
type EventType = domain.EventType

const (
	EventTypeUserMessage               = domain.EventTypeUserMessage
	EventTypeInstructorThought         = domain.EventTypeInstructorThought
	EventTypeInstructorStrategyChanged = domain.EventTypeInstructorStrategyChanged
	EventTypeAgentHandoff              = domain.EventTypeAgentHandoff
)

// Re-export DomainEvent type
type DomainEvent = domain.DomainEvent

// Re-export event constructors
var (
	NewInstructorThoughtEvent         = domain.NewInstructorThoughtEvent
	NewInstructorStrategyChangedEvent = domain.NewInstructorStrategyChangedEvent
	NewAgentHandoffEvent              = domain.NewAgentHandoffEvent
	GenerateEventID                   = domain.GenerateEventID
)
