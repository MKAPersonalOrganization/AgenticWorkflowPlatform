package agent

import "context"

// Rule defines constraints for a specific tool in the current turn.
type Rule struct {
	// TargetTool is the name of the tool this rule applies to.
	TargetTool string
	// Allowed indicates if the tool can be used. If false, it's removed from available tools.
	Allowed bool
	// Forced indicates if the agent MUST use this tool in the current turn.
	Forced bool
	// PreventStop indicates if the agent is forbidden from finishing the task right now.
	PreventStop bool
}

// Requirement is a condition evaluated before each LLM turn.
// It returns a list of Rules that constrain the agent's behavior for that specific turn.
type Requirement interface {
	// Run evaluates the requirement against the current state of the Invocation.
	Run(ctx context.Context, inv *Invocation) ([]Rule, error)
}
