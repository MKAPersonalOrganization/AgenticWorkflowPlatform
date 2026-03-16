package toolreqs

import "context"

// =============================================================================
// ToolRequirement — The extensibility point for new requirement types
// =============================================================================

// RequirementKind identifies the category of a tool requirement.
// Each kind represents a distinct concern (permission, confirmation, etc.).
type RequirementKind string

const (
	// RequirementPermission checks whether a tool/toolset is enabled.
	RequirementPermission RequirementKind = "permission"

	// RequirementConfirmation checks whether user approval is needed.
	RequirementConfirmation RequirementKind = "confirmation"

	// RequirementConditional evaluates conversation-state constraints.
	RequirementConditional RequirementKind = "conditional"
)

// ToolRequirement evaluates a single requirement for a specific tool call.
//
// Implementations are composable — the [Evaluator] aggregates results from
// all registered requirements into a unified [ToolDecision].
//
// To add a new requirement type:
//  1. Define a new RequirementKind constant
//  2. Implement this interface
//  3. Register via [Service.RegisterRequirement]
type ToolRequirement interface {
	// Kind returns the category of this requirement.
	Kind() RequirementKind

	// Evaluate inspects the request context and returns a result.
	// Implementations must be safe for concurrent use.
	Evaluate(ctx context.Context, req *ToolEvalRequest) (*RequirementResult, error)
}

// RequirementResult is the outcome of evaluating a single [ToolRequirement].
type RequirementResult struct {
	// Satisfied is true when the requirement does not block the tool.
	Satisfied bool

	// Action is the specific action dictated by this requirement.
	Action Action

	// Reason is a human-readable explanation (shown to the LLM or user).
	Reason string

	// Priority controls conflict resolution when multiple requirements
	// produce conflicting actions. Higher priority wins for Force/Hide.
	// Block always wins regardless of priority.
	Priority int

	// Confirmation is non-nil when Action is ActionConfirm.
	Confirmation *ConfirmationRequest
}
