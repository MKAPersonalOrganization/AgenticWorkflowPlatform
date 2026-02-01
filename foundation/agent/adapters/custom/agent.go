package custom

import (
	"context"
	"iter"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// AgentAdapter adapts core.AgentConfig to the agent.Agent interface.
type AgentAdapter struct {
	config core.AgentConfig
}

// NewAgentAdapter creates an adapter from an AgentConfig.
func NewAgentAdapter(config core.AgentConfig) agent.Agent {
	return &AgentAdapter{config: config}
}

// Name returns the agent's identifier.
func (a *AgentAdapter) Name() string {
	return a.config.Name
}

// Description returns the agent's system prompt as description.
func (a *AgentAdapter) Description() string {
	return a.config.SystemPrompt
}

// Run executes the agent (not directly supported - use RunnerAdapter instead).
// This method is a placeholder since AgentConfig doesn't have a standalone Run method.
func (a *AgentAdapter) Run(ctx context.Context, inv *agent.Invocation) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		// AgentConfig doesn't support standalone execution
		// Use RunnerAdapter.Run() instead which uses the engine
		yield(nil, &NotImplementedError{"AgentAdapter.Run - use RunnerAdapter instead"})
	}
}

// Tools returns the agent's available tools.
func (a *AgentAdapter) Tools() []agent.Tool {
	tools := make([]agent.Tool, 0, len(a.config.Tools))
	for _, t := range a.config.Tools {
		tools = append(tools, NewToolAdapter(t))
	}
	return tools
}

// SubAgents returns child agents from handoffs.
func (a *AgentAdapter) SubAgents() []agent.Agent {
	// AgentConfig doesn't directly support sub-agents, but has handoffs
	// This could be enhanced to return agents that can be handed off to
	return []agent.Agent{}
}

// GetConfig returns the underlying AgentConfig (for internal use).
func (a *AgentAdapter) GetConfig() core.AgentConfig {
	return a.config
}

// NotImplementedError represents a feature that is not implemented.
type NotImplementedError struct {
	Feature string
}

func (e *NotImplementedError) Error() string {
	return "not implemented: " + e.Feature
}
