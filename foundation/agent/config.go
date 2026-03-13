package agent

// Config defines the configuration for an agent.
//
// Config is the central description of an agent's identity, behavior, and
// capabilities. It is framework-agnostic — adapters translate it to their
// native configuration (ADK-Go's llmagent.Config, custom engine's
// core.AgentConfig, etc.).
//
// Layering:
//   - Identity:       Name, Description
//   - Behavior:       SystemPrompt, InstructionFunc, Requirements
//   - Capabilities:   Tools, SubAgents
//   - Execution:      MaxIterations, OutputSchema
//   - Interception:   Hooks (domain-level), Middlewares (operational)
type Config struct {
	// Name is the unique identifier for the agent.
	// Must be unique within the agent tree. Cannot be "user" (reserved).
	Name string

	// Description is a human-readable description of the agent.
	// Used by LLMs to determine whether to delegate to this agent.
	Description string

	// SystemPrompt is the instructions for how the agent should behave.
	// Supports template variables {state_key} that are resolved at runtime
	// from session state.
	SystemPrompt string

	// InstructionFunc provides instructions that depend on runtime state.
	// When set, it is called before each agent run and appended to SystemPrompt.
	InstructionFunc func(inv *Invocation) (string, error)

	// Tools are the available capabilities for this agent.
	Tools []Tool

	// SubAgents are child agents that this agent can delegate tasks to.
	// Transfer between parent, children, and peers is automatically enabled.
	SubAgents []Agent

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
	// Default is 10. Set to 0 for unlimited (loop until escalate).
	MaxIterations int

	// OutputSchema forces the agent to produce structured JSON output
	// conforming to this schema.
	OutputSchema *Schema

	// ── Lifecycle Hooks ─────────────────────────────────────────────────
	//
	// Hooks are domain-level interception points that can inspect, modify,
	// or short-circuit agent operations. They run at discrete lifecycle
	// moments and communicate via return values (not wrapping).
	//
	// Evaluated sequentially with early-exit semantics: if a hook returns
	// non-nil content, subsequent hooks and the operation itself are skipped.

	// BeforeRunHooks are called before the agent starts its run.
	BeforeRunHooks []BeforeRunHook

	// AfterRunHooks are called after the agent completes its run.
	AfterRunHooks []AfterRunHook

	// BeforeToolHooks are called before each tool execution.
	BeforeToolHooks []BeforeToolHook

	// AfterToolHooks are called after each tool execution.
	AfterToolHooks []AfterToolHook

	// BeforeModelHooks are called before each LLM call.
	BeforeModelHooks []BeforeModelHook

	// AfterModelHooks are called after each LLM call.
	AfterModelHooks []AfterModelHook

	// ── Middlewares ─────────────────────────────────────────────────────
	//
	// Middlewares are operational cross-cutting concerns (logging, tracing,
	// metrics, recovery, rate limiting). They wrap execution via the
	// Handler chain pattern — each middleware calls next.Handle(ctx).
	//
	// Unlike hooks, middlewares cannot short-circuit with a domain result;
	// they observe and decorate the execution pipeline.
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
