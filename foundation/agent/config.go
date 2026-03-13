package agent

// Config defines the configuration for an agent.
//
// Config is the central description of an agent's identity, behaviour, and
// capabilities. It is framework-agnostic — adapters translate it to their
// native configuration (ADK-Go's llmagent.Config, etc.).
//
// Layering:
//   - Identity:     Name, Description
//   - Behaviour:    SystemPrompt, InstructionFunc, Requirements
//   - Capabilities: Tools, SubAgents
//   - Execution:    MaxIterations, OutputSchema
//   - Observability: FunctionMiddlewares (tool-call cross-cutting concerns)
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

	// Tools are the available capabilities for this agent.
	Tools []Tool

	// SubAgents are child agents that this agent can delegate tasks to.
	SubAgents []Agent

	// Requirements are declarative constraints evaluated before each LLM turn.
	// Requirements that implement ToolProvider automatically contribute tools.
	// Requirements that implement PromptProvider automatically contribute to the system prompt.
	// Requirements that implement FunctionMiddlewareProvider contribute to the tool pipeline.
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

	// OutputSchema forces the agent to produce structured JSON output
	// conforming to this schema.
	OutputSchema *Schema

	// FunctionMiddlewares are cross-cutting concerns run around each tool execution
	// (e.g. logging, rate-limiting, argument validation, caching).
	// These are bridged to ADK-Go's BeforeToolCallback / AfterToolCallback.
	FunctionMiddlewares []Middleware
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
