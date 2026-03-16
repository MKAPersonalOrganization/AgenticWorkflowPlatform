// Package toolreqs is the exclusive authority for all tool requirements.
//
// It unifies permission checks, user confirmation, conditional constraints,
// and any future requirement types (rate limits, cost budgets, etc.) into
// a single evaluation pipeline.
//
// # Architecture
//
// The module follows the modular-monolith pattern:
//   - Inbound ports: [Service] exposes the public API
//   - Persistence: [Repository] for PostgreSQL-backed permission storage
//   - Domain: [ToolRequirement], [ToolDecision], [ConfirmationPolicy]
//
// Every tool access decision flows through [Service.EvaluateTool] or
// [Service.GetEnabledToolSets]. No other module should make direct
// permission or requirement decisions.
//
// # Extensibility
//
// New requirement types implement [ToolRequirement] and are registered
// via [Service.RegisterRequirement]. The [Evaluator] aggregates all
// registered requirements into a single [ToolDecision].
//
// Inspired by ADK-Go's tool confirmation flow and AgentStack's
// demand/fulfillment pattern, adapted for our modular monolith.
package toolreqs

// Action represents the outcome of evaluating a single requirement.
type Action string

const (
	// ActionAllow means the requirement is satisfied — the tool may proceed.
	ActionAllow Action = "allow"

	// ActionBlock is a hard veto — the tool must not be used.
	// Any single block overrides all allow/force decisions.
	ActionBlock Action = "block"

	// ActionConfirm means the tool requires user confirmation before execution.
	ActionConfirm Action = "confirm"

	// ActionForce means the tool MUST be used on this turn.
	ActionForce Action = "force"

	// ActionHide means the tool should be silently omitted from the LLM's view.
	ActionHide Action = "hide"
)

// ToolDecision is the aggregate outcome after evaluating ALL requirements
// for a specific tool. Produced by the [Evaluator].
type ToolDecision struct {
	// ToolName identifies the tool this decision applies to.
	ToolName string

	// Allowed is true when no requirement blocks the tool.
	Allowed bool

	// Blocked is true when any requirement hard-vetoes the tool.
	Blocked bool

	// Hidden is true when the tool should be silently omitted.
	Hidden bool

	// Forced is true when the tool must be used this turn.
	Forced bool

	// PreventStop is true when the agent must not stop yet.
	PreventStop bool

	// NeedsConfirm is true when user confirmation is required at execution time.
	NeedsConfirm bool

	// Confirmation carries the confirmation request details (non-nil when NeedsConfirm is true).
	Confirmation *ConfirmationRequest

	// Reasons collects human-readable explanations from all requirements.
	Reasons []string
}

// ConfirmationRequest is emitted when a tool requires user approval before execution.
// This follows the ADK-Go pattern where the runtime pauses and waits for user input.
type ConfirmationRequest struct {
	// ToolName identifies which tool needs approval.
	ToolName string `json:"tool_name"`

	// Hint is a human-readable prompt shown to the user.
	Hint string `json:"hint"`

	// Payload carries application-specific context for the confirmation UI.
	Payload any `json:"payload,omitempty"`
}

// ConfirmationResponse carries the user's approval or rejection decision.
type ConfirmationResponse struct {
	// ToolName identifies which tool was confirmed/rejected.
	ToolName string `json:"tool_name"`

	// Confirmed is true when the user approved the tool execution.
	Confirmed bool `json:"confirmed"`

	// Payload carries application-specific data from the confirmation UI
	// (e.g., modified parameters, approved scope).
	Payload any `json:"payload,omitempty"`
}

// ToolEvalRequest contains all context needed to evaluate requirements
// for a specific tool call.
type ToolEvalRequest struct {
	// UserID is the user making the request.
	UserID string

	// AgentID is the agent attempting to use the tool.
	AgentID string

	// ToolSetID is the tool set the tool belongs to (may be empty for static tools).
	ToolSetID string

	// ToolID is the unique identifier of the tool within its set.
	ToolID string

	// ToolName is the display/execution name of the tool.
	ToolName string

	// Args are the arguments the LLM provided for this tool call.
	// Available at execution time, nil during resolution.
	Args map[string]any
}
