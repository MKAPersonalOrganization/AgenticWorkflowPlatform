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

	// SubAgents are child agents for hierarchical delegation.
	SubAgents []Agent

	// Requirements are declarative constraints evaluated before each LLM turn.
	// Requirements that implement ToolProvider automatically contribute tools.
	// Requirements that implement PromptProvider automatically contribute to the system prompt.
	// The RequirementMiddleware is automatically wired when Requirements is non-empty.
	//
	// Example:
	//
	//    Requirements: []agent.Requirement{
	//        agent.NewRequireUserInput(),
	//        agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),
	//        agent.Require("custom", func(ctx context.Context, inv *agent.Invocation) ([]agent.Rule, error) {
	//            return []agent.Rule{{TargetTool: "search", Blocked: true}}, nil
	//        }),
	//    },
	Requirements []Requirement

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

	// Middlewares are applied to this agent's operation stages.
	AgentMiddlewares    []Middleware // Runs around the entire agent run
	ChatMiddlewares     []Middleware // Runs around each LLM call
	FunctionMiddlewares []Middleware // Runs around each tool execution

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
//
// For tool-access constraints, quiz tools, and human-in-the-loop flows,
// use the declarative Requirements system on Config instead.
type Capabilities struct {
	// LongTermMemory enables access to the user's long-term memory store.
	LongTermMemory bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig(name string) Config {
	return Config{
		Name:          name,
		MaxIterations: 10,
	}
}

// CollectRequirementTools returns all tools provided by Requirements
// that implement ToolProvider.
func (cfg *Config) CollectRequirementTools() []ExecutableTool {
	var tools []ExecutableTool
	for _, req := range cfg.Requirements {
		if tp, ok := req.(ToolProvider); ok {
			tools = append(tools, tp.ProvidedTools()...)
		}
	}
	return tools
}

// CollectRequirementPrompts returns concatenated prompt additions from
// Requirements that implement PromptProvider.
func (cfg *Config) CollectRequirementPrompts() string {
	var additions string
	for _, req := range cfg.Requirements {
		if pp, ok := req.(PromptProvider); ok {
			additions += "\n\n" + pp.PromptAddition()
		}
	}
	return additions
}
