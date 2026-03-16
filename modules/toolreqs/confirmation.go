package toolreqs

import (
	"context"
	"fmt"
	"sync"
)

// =============================================================================
// ConfirmationRequirement — ADK-Go style user confirmation
// =============================================================================

// ConfirmationPolicy defines when user confirmation is required for a tool.
//
// Policies can be static (AlwaysConfirm) or dynamic (ConfirmFunc).
// They are configured per-tool, per-agent, or globally.
type ConfirmationPolicy struct {
	// AlwaysConfirm forces confirmation for every invocation of this tool.
	AlwaysConfirm bool

	// ConfirmFunc is an optional dynamic check based on tool name and arguments.
	// When non-nil, it takes precedence over AlwaysConfirm.
	//
	// Example: confirm only when a "delete" argument is true:
	//
	//   ConfirmFunc: func(toolName string, args map[string]any) bool {
	//       del, _ := args["delete"].(bool)
	//       return del
	//   }
	ConfirmFunc func(toolName string, args map[string]any) bool

	// Hint is a human-readable prompt shown to the user.
	// If empty, a default prompt is generated.
	Hint string
}

// ConfirmationRequirement checks whether a tool call needs user confirmation.
//
// It supports three levels of policy resolution (highest priority first):
//  1. Global policies — tools that ALWAYS require confirmation regardless of agent.
//  2. Agent-specific policies — per-agent, per-tool configuration.
//  3. Default policies — fallback for all agents.
//
// This follows the ADK-Go pattern:
//   - Resolution phase: [Evaluate] returns ActionConfirm (tool is available but needs approval).
//   - Execution phase: [Middleware] intercepts the call and invokes io.Confirm().
type ConfirmationRequirement struct {
	mu sync.RWMutex

	// defaults are tool policies that apply when no agent-specific policy exists.
	defaults map[string]*ConfirmationPolicy

	// agents maps agentID → toolName → policy for agent-specific overrides.
	agents map[string]map[string]*ConfirmationPolicy

	// globals are tools that ALWAYS require confirmation, regardless of agent config.
	// These are admin-level enforcements (e.g., dangerous tools like file_delete).
	globals map[string]*ConfirmationPolicy
}

// NewConfirmationRequirement creates a new confirmation requirement with empty policies.
func NewConfirmationRequirement() *ConfirmationRequirement {
	return &ConfirmationRequirement{
		defaults: make(map[string]*ConfirmationPolicy),
		agents:   make(map[string]map[string]*ConfirmationPolicy),
		globals:  make(map[string]*ConfirmationPolicy),
	}
}

// Kind implements [ToolRequirement].
func (r *ConfirmationRequirement) Kind() RequirementKind { return RequirementConfirmation }

// =============================================================================
// Policy management
// =============================================================================

// SetGlobalPolicy configures a tool that ALWAYS requires confirmation.
// This is the "force user confirmation for certain tools" capability.
//
// Global policies cannot be overridden by agent-specific policies.
func (r *ConfirmationRequirement) SetGlobalPolicy(toolName string, policy *ConfirmationPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.globals[toolName] = policy
}

// RemoveGlobalPolicy removes a global confirmation policy.
func (r *ConfirmationRequirement) RemoveGlobalPolicy(toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.globals, toolName)
}

// SetDefaultPolicy configures confirmation for a tool across all agents.
func (r *ConfirmationRequirement) SetDefaultPolicy(toolName string, policy *ConfirmationPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaults[toolName] = policy
}

// RemoveDefaultPolicy removes a default confirmation policy.
func (r *ConfirmationRequirement) RemoveDefaultPolicy(toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.defaults, toolName)
}

// SetAgentPolicy configures confirmation for a specific tool on a specific agent.
func (r *ConfirmationRequirement) SetAgentPolicy(agentID, toolName string, policy *ConfirmationPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.agents[agentID] == nil {
		r.agents[agentID] = make(map[string]*ConfirmationPolicy)
	}
	r.agents[agentID][toolName] = policy
}

// RemoveAgentPolicy removes an agent-specific confirmation policy.
func (r *ConfirmationRequirement) RemoveAgentPolicy(agentID, toolName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if agentPolicies, ok := r.agents[agentID]; ok {
		delete(agentPolicies, toolName)
		if len(agentPolicies) == 0 {
			delete(r.agents, agentID)
		}
	}
}

// =============================================================================
// Evaluation
// =============================================================================

// RequiresConfirmation checks whether a tool requires user confirmation.
// This is used at execution time (in the middleware) to decide whether
// to invoke io.Confirm().
func (r *ConfirmationRequirement) RequiresConfirmation(agentID, toolName string, args map[string]any) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.requiresConfirmationLocked(agentID, toolName, args)
}

func (r *ConfirmationRequirement) requiresConfirmationLocked(agentID, toolName string, args map[string]any) bool {
	// 1. Global policies always win — admin-level enforcement.
	if p, ok := r.globals[toolName]; ok {
		return policyMatches(p, toolName, args)
	}

	// 2. Agent-specific policies.
	if agentPolicies, ok := r.agents[agentID]; ok {
		if p, ok := agentPolicies[toolName]; ok {
			return policyMatches(p, toolName, args)
		}
	}

	// 3. Default policies.
	if p, ok := r.defaults[toolName]; ok {
		return policyMatches(p, toolName, args)
	}

	return false
}

// Evaluate implements [ToolRequirement].
//
// At resolution time (Args is nil), it checks if the tool has any confirmation
// policy configured and marks it as needing confirmation. The actual enforcement
// happens at execution time via the [Middleware].
func (r *ConfirmationRequirement) Evaluate(ctx context.Context, req *ToolEvalRequest) (*RequirementResult, error) {
	if !r.RequiresConfirmation(req.AgentID, req.ToolName, req.Args) {
		return &RequirementResult{Satisfied: true, Action: ActionAllow}, nil
	}

	hint := r.getHint(req.AgentID, req.ToolName)

	return &RequirementResult{
		Satisfied: false,
		Action:    ActionConfirm,
		Reason:    hint,
		Confirmation: &ConfirmationRequest{
			ToolName: req.ToolName,
			Hint:     hint,
		},
	}, nil
}

// getHint resolves the confirmation hint for a tool, following the same
// priority chain as policy resolution: global → agent → default → generated.
func (r *ConfirmationRequirement) getHint(agentID, toolName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if p, ok := r.globals[toolName]; ok && p.Hint != "" {
		return p.Hint
	}
	if agentPolicies, ok := r.agents[agentID]; ok {
		if p, ok := agentPolicies[toolName]; ok && p.Hint != "" {
			return p.Hint
		}
	}
	if p, ok := r.defaults[toolName]; ok && p.Hint != "" {
		return p.Hint
	}
	return fmt.Sprintf("Allow tool '%s' to execute?", toolName)
}

// policyMatches evaluates whether a confirmation policy requires confirmation.
func policyMatches(p *ConfirmationPolicy, toolName string, args map[string]any) bool {
	if p.ConfirmFunc != nil {
		return p.ConfirmFunc(toolName, args)
	}
	return p.AlwaysConfirm
}

// Verify interface compliance.
var _ ToolRequirement = (*ConfirmationRequirement)(nil)
