package eventbus

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents"
)

// AgentContract defines the event-driven contract for an agent.
// Agents declare what events they listen to and what handlers process those events.
type AgentContract struct {
	// Identity
	Name string // Unique agent name, e.g., "math-instructor"

	// Event Contract
	ListensTo []agents.EventType // Events this agent listens to (subscriptions)

	// Event Handlers - map event types to handler functions
	// Handlers are invoked when events are received and can modify the agent's behavior
	Handlers agents.EventHandlerRegistry

	// Tools - domain-specific tools only (domain-specific tools for this agent)
	Tools []core.Tool

	// Factory function to create the agent
	Factory AgentFactory
}

// AgentFactory creates an agent configuration with a given notifier and whiteboard projection
type AgentFactory func(notifier core.EventNotifier, whiteboardProjection interface{ Get(threadID string) string }) core.AgentConfig

// EventTypeSlug extracts the slug from an event type.
// e.g., "question.math" -> "math"
func EventTypeSlug(et agents.EventType) string {
	s := string(et)
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}
