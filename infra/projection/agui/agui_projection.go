package agui

import (
	"school-with-ai/domain"
	"school-with-ai/infra/eventstore"
)

// EventEmitter is a function that emits real-time events to connected clients.
type EventEmitter func(threadID string, event any) error

// Re-export types from domain
type (
	AguiProjection    = domain.AguiProjection
	AguiThreadState   = domain.AguiThreadState
	AguiRunState      = domain.AguiRunState
	AguiToolCall      = domain.AguiToolCall
	AguiAgentActivity = domain.AguiAgentActivity
)

// AguiProjectionImpl is an implementation of AguiProjection.
// We keep the implementation in infra, but the interface is in domain.
type AguiProjectionImpl interface {
	AguiProjection
	eventstore.Projection
}
