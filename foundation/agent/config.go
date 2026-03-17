package agent

// TransferPolicy controls how an LLM agent delegates to other agents.
// Maps directly to ADK-Go's transfer_to_agent tool behaviour.
type TransferPolicy struct {
	// DisallowTransferToParent prevents the agent from transferring back
	// to its parent agent. Default false (transfer allowed).
	DisallowTransferToParent bool

	// DisallowTransferToPeers prevents the agent from transferring to
	// sibling agents (peers under the same parent). Default false.
	DisallowTransferToPeers bool
}

// IncludeContents controls whether conversation history is forwarded
// to sub-agents when they run within a workflow.
type IncludeContents string

const (
	// IncludeContentsDefault forwards full conversation history (default).
	IncludeContentsDefault IncludeContents = "default"

	// IncludeContentsNone starts the sub-agent with a blank context.
	IncludeContentsNone IncludeContents = "none"
)

// Config defines the configuration for an agent.
//
// Config is the central description of an agent's identity, behaviour, and
// capabilities. It is framework-agnostic — adapters translate it to their
// native configuration format.
//
// Layering:
//   - Identity:     Name, Description
//   - Behaviour:    SystemPrompt, InstructionFunc, Requirements
//   - Capabilities: Tools, SubAgents
//   - Transfer:     TransferPolicy (parent/peer delegation control)
//   - I/O Schema:   InputSchema, OutputSchema
//   - Execution:    MaxIterations, OutputKey, IncludeContents
//   - Global:       GlobalInstruction (shared across agent tree)
type Config struct {
	// Name is the unique identifier for the agent.
	// Must be unique within the agent tree. Cannot be "user" (reserved).
	Name string

	// Description is a human-readable description of the agent.
	// Used by LLMs to determine whether to delegate to this agent.
	Description string

	// SystemPrompt is the instructions for how the agent should behave.
	SystemPrompt string

	// InstructionFunc provides instructions that depend on runtime state.
	// Called before each agent run and appended to SystemPrompt.
	InstructionFunc func(inv *Invocation) (string, error)

	// GlobalInstruction is shared across the entire agent tree.
	// Only effective on the root agent — propagated to all descendants
	// by the ADK-Go runtime. Use for organisation-wide policies.
	GlobalInstruction string

	// Tools are the available capabilities for this agent.
	Tools []Tool

	// SubAgents are child agents that this agent can delegate tasks to.
	SubAgents []Agent

	// Requirements are declarative constraints evaluated before each LLM turn.
	// Requirements that implement ToolProvider automatically contribute tools.
	// Requirements that implement PromptProvider automatically contribute to the system prompt.
	//
	// Example:
	//
	//    Requirements: []agent.Requirement{
	//        agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),
	//    },
	Requirements []Requirement

	// MaxIterations limits the number of turns in a single run.
	// Default is 10. Set to 0 for unlimited.
	MaxIterations int

	// ── Multi-Agent Orchestration ──

	// TransferPolicy controls delegation to parent and peer agents.
	// Maps to ADK-Go's DisallowTransferToParent / DisallowTransferToPeers.
	TransferPolicy TransferPolicy

	// IncludeContents controls whether conversation history is forwarded
	// to this agent when it runs as a sub-agent in a workflow.
	// Default is "default" (include full history).
	IncludeContents IncludeContents

	// OutputKey optionally stores this agent's final text output in
	// session state under this key, making it available to subsequent
	// agents via {output_key} template syntax in their instructions.
	OutputKey string

	// ── Schema ──

	// InputSchema defines the expected input when this agent is invoked
	// as a tool (via AgentTool). Informs the caller about required args.
	InputSchema *Schema

	// OutputSchema forces the agent to produce structured JSON output
	// conforming to this schema. When set, tool use is disabled.
	OutputSchema *Schema

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
