package agents

// This file re-exports domain helpers for backward compatibility
// New code should import from domain directly

import (
	"school-with-ai/domain"
)

// Re-export DomainEvent type
type DomainEvent = domain.DomainEvent

// Re-export EventType
type EventType = domain.EventType

// Re-export event type constants
const (
	EventTypeUserMessage = domain.EventTypeUserMessage
)

// Re-export helpers
var (
	GenerateEventID = domain.GenerateEventID
)
