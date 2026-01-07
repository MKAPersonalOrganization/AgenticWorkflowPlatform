package eventbus

import (
	"school-with-ai/sdk/agentsdk/core"
)

// AgentContract defines the contract for an agent.
type AgentContract struct {
	// Identity
	Name string // Unique agent name, e.g., "math-instructor"

	// Tools - domain-specific tools for this agent
	Tools []core.Tool

	// Factory function to create the agent
	Factory AgentFactory
}

// AgentFactory creates an agent configuration with a given notifier and whiteboard projection
type AgentFactory func(notifier core.EventNotifier, whiteboardProjection interface{ Get(threadID string) string }) core.AgentConfig
