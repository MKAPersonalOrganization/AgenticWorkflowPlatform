package agent

import "context"

// =============================================================================
// Rule — A single constraint on a tool for the current LLM turn
// =============================================================================

// Rule defines a declarative constraint for a specific tool.
//
// Rules are produced by Requirements and aggregated by the Reasoner.
// When multiple rules target the same tool, the Reasoner resolves conflicts
// using Priority (higher wins for Forced/Hidden; Blocked=true always wins).
//
// Go zero-value semantics: a default Rule{TargetTool: "x"} means
// "no constraint" — the tool is allowed, not forced, not hidden.
type Rule struct {
	// TargetTool is the name of the tool this rule applies to.
	TargetTool string

	// Blocked indicates the tool cannot be used. true = remove from available tools.
	// This is a hard veto: if ANY rule says Blocked=true, the tool is blocked
	// regardless of priority.
	Blocked bool

	// Forced indicates the agent MUST use this tool on the current turn.
	// Resolved by Priority when multiple rules conflict.
	Forced bool

	// Hidden indicates the tool should be silently omitted from the LLM's view.
	// Unlike Blocked, Hidden tools don't generate "CRITICAL SYSTEM" messages.
	Hidden bool

	// PreventStop indicates the agent is forbidden from finishing right now.
	PreventStop bool

	// Reason is a human-readable explanation shown to the LLM.
	// e.g. "You must verify understanding before proceeding."
	Reason string

	// Priority controls conflict resolution. Higher priority wins.
	// Default is 0. Requirements set this on their rules.
	Priority int
}

// =============================================================================
// Requirement — The core interface
// =============================================================================

// Requirement is a declarative constraint evaluated before each LLM turn.
// It inspects the current invocation state and returns Rules that constrain
// the agent's behavior for that specific turn.
//
// Requirements are the building blocks of the declarative agent system.
// They replace imperative middleware for tool-access control, ordering,
// and human-in-the-loop flows.
//
// Optional interfaces a Requirement can implement:
//   - ToolProvider: provides additional tools to the agent
//   - PromptProvider: contributes to the system prompt
//   - Enableable: can be dynamically enabled/disabled
type Requirement interface {
	// Run evaluates the requirement and returns rules for the current turn.
	Run(ctx context.Context, inv *Invocation) ([]Rule, error)
}

// =============================================================================
// Optional interfaces for Requirements
// =============================================================================

// ToolProvider is implemented by Requirements that inject additional tools
// into the agent's tool set (e.g., RequireUserInput provides the ask_user tool).
type ToolProvider interface {
	// ProvidedTools returns tools that should be added to the agent.
	ProvidedTools() []ExecutableTool
}

// PromptProvider is implemented by Requirements that contribute to the
// agent's system prompt (e.g., instructions about how to use quiz tools).
type PromptProvider interface {
	// PromptAddition returns text to append to the system prompt.
	PromptAddition() string
}

// Enableable is implemented by Requirements that can be dynamically
// enabled or disabled. Disabled requirements are skipped by the Reasoner.
type Enableable interface {
	// Enabled returns whether this requirement should be evaluated.
	Enabled() bool
}

// =============================================================================
// RequirementFunc — Functional shorthand (BeeAI's @requirement decorator)
// =============================================================================

// RequirementFunc adapts a function to the Requirement interface.
// This is the Go equivalent of BeeAI's @requirement decorator, enabling
// inline requirement definitions without creating dedicated structs.
//
// Example:
//
//	agent.Require("block-search-early", func(ctx context.Context, inv *agent.Invocation) ([]agent.Rule, error) {
//	    if countSteps(inv) < 3 {
//	        return []agent.Rule{{TargetTool: "search", Blocked: true, Reason: "Think first"}}, nil
//	    }
//	    return nil, nil
//	})
type RequirementFunc struct {
	name string
	fn   func(ctx context.Context, inv *Invocation) ([]Rule, error)
}

// Require creates a RequirementFunc from a function.
// This is the primary way to create ad-hoc requirements.
func Require(name string, fn func(ctx context.Context, inv *Invocation) ([]Rule, error)) *RequirementFunc {
	return &RequirementFunc{name: name, fn: fn}
}

func (r *RequirementFunc) Run(ctx context.Context, inv *Invocation) ([]Rule, error) {
	return r.fn(ctx, inv)
}

func (r *RequirementFunc) String() string { return r.name }

// Verify interface compliance.
var _ Requirement = (*RequirementFunc)(nil)

// =============================================================================
// RequirementRequest — Resolved output from the Reasoner
// =============================================================================

// RequirementRequest is the resolved output produced by the Reasoner after
// evaluating all Requirements and resolving conflicts. It represents the
// final, coherent set of constraints for one LLM turn.
type RequirementRequest struct {
	// AllowedTools are the tools the LLM may call (after filtering).
	AllowedTools []Tool

	// HiddenTools are tools silently omitted from the LLM's view.
	// They differ from blocked tools: no "CRITICAL" message is injected.
	HiddenTools []string

	// ToolChoice controls which tool the LLM must use.
	// Values: "" (auto), "required", or a specific tool name.
	ToolChoice string

	// CanFinish indicates whether the agent is allowed to produce a final answer.
	// false when any rule sets PreventStop=true.
	CanFinish bool

	// Reasons maps tool names to human-readable explanations.
	// These are included in injected system messages.
	Reasons map[string]string

	// ForcedTools lists tools that must be used this turn.
	ForcedTools []string
}
