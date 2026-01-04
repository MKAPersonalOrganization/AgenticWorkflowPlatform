package sdk

import "school-with-ai/services/llm"

// EventNotifier is the interface for handling domain events from agents
type EventNotifier interface {
	HandleDomainEvent(event Event) error
}

// AgentConfig defines the configuration for an agent
type AgentConfig struct {
	// Identity
	Name string // Unique identifier for the agent

	// Behavior
	SystemPrompt string // Instructions for how the agent should behave
	Tools        []Tool // Available tools for this agent (implements Tool interface)

	// Event handling
	Notifier EventNotifier // UI/Event notifier for this agent
}

// DefaultConfig returns a basic agent configuration
func DefaultConfig(name string) AgentConfig {
	return AgentConfig{
		Name:         name,
		SystemPrompt: "You are a helpful assistant.",
		Tools:        []Tool{},
	}
}

// ToAITools converts agent.Tool slice to llm.AITool slice for ChatClient
func ToAITools(tools []Tool) []llm.AITool {
	result := make([]llm.AITool, len(tools))
	for i, t := range tools {
		result[i] = llm.NewAIFunctionTool(
			t.Name(),
			t.Description(),
			t.Parameters(),
		)
	}
	return result
}

// ToChatOptions builds ChatOptions from agent configuration
func ToChatOptions(config AgentConfig) *llm.ChatOptions {
	return &llm.ChatOptions{
		Instructions: config.SystemPrompt,
		Tools:        ToAITools(config.Tools),
		ToolMode:     llm.ToolModeAuto,
	}
}
