package toolreqs

import "context"

// =============================================================================
// Evaluator — aggregates all requirements into a single ToolDecision
// =============================================================================

// Evaluator evaluates all registered [ToolRequirement]s for a tool call and
// aggregates their results into a single [ToolDecision].
//
// Conflict resolution rules:
//   - Block always wins: if ANY requirement says Block, the tool is blocked.
//   - Confirm is additive: if any requirement says Confirm, the decision
//     includes NeedsConfirm (but the tool is still Allowed for resolution).
//   - Force is resolved by Priority: highest priority Force wins.
//   - Hide is resolved by Priority: highest priority Hide wins.
//   - PreventStop from any requirement is preserved.
type Evaluator struct {
	requirements []ToolRequirement
}

// NewEvaluator creates an evaluator with the given requirements.
func NewEvaluator(requirements ...ToolRequirement) *Evaluator {
	return &Evaluator{requirements: requirements}
}

// Evaluate runs all requirements and produces an aggregate [ToolDecision].
func (e *Evaluator) Evaluate(ctx context.Context, req *ToolEvalRequest) (*ToolDecision, error) {
	decision := &ToolDecision{
		ToolName: req.ToolName,
		Allowed:  true, // optimistic default
	}

	var (
		highestForcePriority int
		highestHidePriority  int
		hasForce             bool
		hasHide              bool
	)

	for _, requirement := range e.requirements {
		result, err := requirement.Evaluate(ctx, req)
		if err != nil {
			// Requirement evaluation errors are non-fatal: log and continue.
			// A failing requirement should not block tool access.
			decision.Reasons = append(decision.Reasons,
				"requirement "+string(requirement.Kind())+" error: "+err.Error())
			continue
		}

		if result == nil {
			continue
		}

		// Collect reasons.
		if result.Reason != "" {
			decision.Reasons = append(decision.Reasons, result.Reason)
		}

		switch result.Action {
		case ActionBlock:
			// Hard veto — any single block wins immediately.
			decision.Allowed = false
			decision.Blocked = true
			decision.Forced = false
			// Don't short-circuit: collect all reasons for diagnostics.

		case ActionHide:
			if !hasHide || result.Priority > highestHidePriority {
				decision.Hidden = true
				hasHide = true
				highestHidePriority = result.Priority
			}

		case ActionForce:
			if !decision.Blocked {
				if !hasForce || result.Priority > highestForcePriority {
					decision.Forced = true
					hasForce = true
					highestForcePriority = result.Priority
				}
			}

		case ActionConfirm:
			// Confirmation doesn't block the tool from being available,
			// but marks it as needing approval at execution time.
			decision.NeedsConfirm = true
			if result.Confirmation != nil {
				decision.Confirmation = result.Confirmation
			}

		case ActionAllow:
			// No-op: tool remains allowed (default).
		}
	}

	// Block overrides Force and Confirm.
	if decision.Blocked {
		decision.Forced = false
		decision.NeedsConfirm = false
		decision.Confirmation = nil
	}

	return decision, nil
}

// AddRequirement appends a requirement to the evaluator.
func (e *Evaluator) AddRequirement(req ToolRequirement) {
	e.requirements = append(e.requirements, req)
}

// Requirements returns the currently registered requirements (read-only view).
func (e *Evaluator) Requirements() []ToolRequirement {
	out := make([]ToolRequirement, len(e.requirements))
	copy(out, e.requirements)
	return out
}
