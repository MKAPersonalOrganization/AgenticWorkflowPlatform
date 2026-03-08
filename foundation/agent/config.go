package agent

// Config defines the configuration for an agent.
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

	// Tools are the available capabilities for this agent.
	Tools []Tool

	// Requirements are declarative constraints evaluated before each LLM turn.
	// Requirements that implement ToolProvider automatically contribute tools.
	// Requirements that implement PromptProvider automatically contribute to the system prompt.
	// The RequirementMiddleware is automatically wired when Requirements is non-empty.
	//
	// Example:
	//
	//    Requirements: []agent.Requirement{
	//        agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),
	//        agent.Require("custom", func(ctx context.Context, inv *agent.Invocation) ([]agent.Rule, error) {
	//            return []agent.Rule{{TargetTool: "search", Blocked: true}}, nil
	//        }),
	//    },
	Requirements []Requirement

	// MaxIterations limits the number of turns in a single run.
	// Default is 10.
	MaxIterations int

	// Middlewares are applied to this agent's operation stages.
	AgentMiddlewares    []Middleware // Runs around the entire agent run
	ChatMiddlewares     []Middleware // Runs around each LLM call
	FunctionMiddlewares []Middleware // Runs around each tool execution
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
