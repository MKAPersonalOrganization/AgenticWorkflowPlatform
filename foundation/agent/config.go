package agent

// Config defines the configuration for an agent.
// This is the framework-agnostic definition used by services.
//
// This config can be used to create agents via adapters:
// - Custom adapter creates agentsdk agents
// - ADK-Go adapter creates llmagent.Agent
// - Blades adapter creates blades.Agent
type Config struct {
	// Name is the unique identifier for the agent.
	// Must be unique within the agent tree.
	Name string

	// Description is a human-readable description of the agent.
	// Used by LLMs to determine whether to delegate to this agent.
	Description string

	// SystemPrompt is the instructions for how the agent should behave.
	// This is used as the Instruction in ADK-Go and Blades.
	SystemPrompt string

	// InstructionProvider generates instructions dynamically.
	// Takes precedence over SystemPrompt if set.
	InstructionProvider func(ctx ReadonlyContext) (string, error)

	// GlobalInstruction is shared across all agents in the tree.
	// Only effective when set on the root agent.
	GlobalInstruction string

	// Tools are the available capabilities for this agent.
	Tools []Tool

	// Toolsets provide grouped tool collections.
	Toolsets []Toolset

	// ToolResolvers provide dynamic tool discovery.
	ToolResolvers []ToolResolver

	// Handoffs declares which agents this agent can transfer control to.
	Handoffs []Handoff

	// SubAgents are child agents for hierarchical delegation.
	SubAgents []Agent

	// Capabilities enables specific agent behaviors.
	Capabilities Capabilities

	// MaxIterations limits the number of turns in a single run.
	// Default is 10.
	MaxIterations int

	// ModelID (optional) overrides the default model.
	ModelID string

	// Model is the LLM client for this agent (optional).
	// If not set, the default model from the runner is used.
	Model Model

	// Middlewares are applied to this agent's handler.
	Middlewares []Middleware

	// BeforeAgentCallbacks run before agent execution.
	BeforeAgentCallbacks []BeforeAgentCallback

	// AfterAgentCallbacks run after agent execution.
	AfterAgentCallbacks []AfterAgentCallback

	// BeforeToolCallbacks run before tool execution.
	BeforeToolCallbacks []BeforeToolCallback

	// AfterToolCallbacks run after tool execution.
	AfterToolCallbacks []AfterToolCallback

	// OutputKey stores the agent's output in session state.
	// Useful for connecting agents to coordinate with each other.
	OutputKey string

	// InputSchema defines the expected input structure (optional).
	// Used when the agent is invoked as a tool.
	InputSchema map[string]any

	// OutputSchema defines the expected output structure (optional).
	// When set, the agent can only reply (no tool calls).
	OutputSchema map[string]any

	// DisallowTransferToParent prevents escalation to parent agent.
	DisallowTransferToParent bool

	// DisallowTransferToPeers prevents transfer to sibling agents.
	DisallowTransferToPeers bool
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

	// CodeExecution enables the agent to execute code.
	CodeExecution bool

	// WebSearch enables the agent to search the web.
	WebSearch bool

	// Artifacts enables the agent to save and load artifacts.
	Artifacts bool
}

// Handoff represents a transfer of control to another agent.
type Handoff struct {
	// AgentName is the target agent's name.
	AgentName string

	// Description explains when to use this handoff.
	Description string

	// InputSchema defines expected input for the handoff (optional).
	InputSchema map[string]any
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(name string) Config {
	return Config{
		Name:          name,
		MaxIterations: 10,
	}
}
