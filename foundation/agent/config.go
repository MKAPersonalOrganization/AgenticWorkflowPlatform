package agent

// Config defines the configuration for an agent.
// This is the framework-agnostic definition used by services.
type Config struct {
	// Name is the unique identifier for the agent.
	Name string

	// Description is a human-readable description of the agent.
	Description string

	// SystemPrompt is the instructions for how the agent should behave.
	SystemPrompt string

	// Tools are the available capabilities for this agent.
	Tools []Tool

	// Handoffs declares which agents this agent can transfer control to.
	Handoffs []Handoff

	// Capabilities enables specific agent behaviors.
	Capabilities Capabilities

	// MaxIterations limits the number of turns in a single run.
	MaxIterations int

	// ModelID (optional) overrides the default model.
	ModelID string
}

// Capabilities defines specific behaviors/features the agent requires.
type Capabilities struct {
	// InputElicitation enables the agent to ask the user for missing information
	// before calling a tool.
	InputElicitation bool

	// LongTermMemory enables access to the user's long-term memory store.
	LongTermMemory bool

	// StudentProfile enables access to the student's learning profile.
	StudentProfile bool
}

// Handoff represents a transfer of control to another agent.
type Handoff struct {
	AgentName   string
	Description string
}
