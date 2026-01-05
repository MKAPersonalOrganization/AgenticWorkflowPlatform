package eventbus

import (
	"school-with-ai/services/agents"
	"school-with-ai/sdk/agent-sdk"
)

// AgentContract defines the event-driven contract for an agent.
// Agents declare what events they listen to and what events they emit.
// The system automatically handles routing and tool generation.
type AgentContract struct {
	// Identity
	Name string // Unique agent name, e.g., "math-instructor"

	// Event Contract
	ListensTo []agents.EventType // Events this agent reacts to (subscriptions)
	Emits     []agents.EventType // Events this agent can emit (publications)

	// Event Handlers - map event types to handler functions
	// Handlers are invoked when events are received and can modify the agent's behavior
	Handlers agents.EventHandlerRegistry

	// Tools - domain-specific tools only (emission tools are auto-generated from Emits)
	Tools []sdk.Tool

	// Factory function to create the agent
	Factory AgentFactory
}

// AgentFactory creates an agent configuration with a given notifier
type AgentFactory func(notifier sdk.EventNotifier) sdk.AgentConfig

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

// EmissionToolName returns the tool name for emitting this event type.
// e.g., "question.math" -> "emit_math_question"
func EmissionToolName(et agents.EventType) string {
	slug := EventTypeSlug(et)
	return "emit_" + slug + "_question"
}

// EventTypeFromToolName extracts the event type from an emission tool name.
// e.g., "emit_math_question" -> "question.math"
func EventTypeFromToolName(toolName string) agents.EventType {
	// "emit_math_question" -> "math"
	if len(toolName) < 14 { // len("emit_") + len("_question")
		return ""
	}
	slug := toolName[5 : len(toolName)-9] // strip "emit_" prefix and "_question" suffix
	return agents.EventType("question." + slug)
}
