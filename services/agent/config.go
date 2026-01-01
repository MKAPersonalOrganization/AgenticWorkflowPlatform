package agent

import "school-with-ai/services/llm"

// AgentConfig defines the configuration for an agent
type AgentConfig struct {
	// Identity
	Name           string // Unique identifier for the agent
	Role           string // "router" | "instructor" | "specialist" (for backward compatibility)
	Specialization string // Domain expertise (e.g., "math", "physics")

	// Behavior
	SystemPrompt string     // Instructions for how the agent should behave
	Tools        []llm.Tool // Available tools for this agent

	// Collaboration (for multi-agent scenarios)
	HasPrivateMemory bool // Whether agent maintains private memory (isolated)
}

// DefaultConfig returns a basic agent configuration
func DefaultConfig(name string) AgentConfig {
	return AgentConfig{
		Name:             name,
		Role:             "instructor",
		Specialization:   "",
		SystemPrompt:     "You are a helpful assistant.",
		Tools:            []llm.Tool{},
		HasPrivateMemory: false,
	}
}
