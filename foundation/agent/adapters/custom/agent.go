package custom

import (
	"context"
	"iter"

	"school-with-ai/foundation/agent"
)

// AgentAdapter adapts agent.Config to the agent.Agent interface.
type AgentAdapter struct {
	config agent.Config
}

// NewAgentAdapter creates an adapter from an AgentConfig.
func NewAgentAdapter(config agent.Config) agent.Agent {
	return &AgentAdapter{config: config}
}

// Name returns the agent's identifier.
func (a *AgentAdapter) Name() string {
	return a.config.Name
}

// Description returns the agent's description.
func (a *AgentAdapter) Description() string {
	if a.config.Description != "" {
		return a.config.Description
	}
	return a.config.SystemPrompt
}

// Run executes the agent (not directly supported - use RunnerAdapter instead).
// This method is a placeholder since Config doesn't have a standalone Run method.
func (a *AgentAdapter) Run(ctx context.Context, inv *agent.Invocation) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		// AgentConfig doesn't support standalone execution
		// Use RunnerAdapter.Run() instead which uses the engine
		yield(nil, &NotImplementedError{"AgentAdapter.Run - use RunnerAdapter instead"})
	}
}

// Tools returns the agent's available tools.
func (a *AgentAdapter) Tools() []agent.Tool {
	return a.config.Tools
}

// SubAgents returns child agents from handoffs.
func (a *AgentAdapter) SubAgents() []agent.Agent {
	// AgentConfig doesn't directly support sub-agents, but has handoffs
	// This could be enhanced to return agents that can be handed off to
	return []agent.Agent{}
}

// GetConfig returns the underlying AgentConfig (for internal use).
func (a *AgentAdapter) GetConfig() agent.Config {
	return a.config
}

// NotImplementedError represents a feature that is not implemented.
type NotImplementedError struct {
	Feature string
}

func (e *NotImplementedError) Error() string {
	return "not implemented: " + e.Feature
}
