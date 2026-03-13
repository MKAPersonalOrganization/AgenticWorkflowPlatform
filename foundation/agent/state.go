package agent

// ──────────────────────────────────────────────────────────────────────────────
// State Management
//
// State changes are NOT applied directly. They are accumulated in a StateDelta
// and applied when the step commits. This gives transactional semantics:
// either all changes from a step are applied, or none are.
// ──────────────────────────────────────────────────────────────────────────────

// StateDelta accumulates state changes during an agent step.
// Changes are applied transactionally when the step completes.
type StateDelta struct {
	delta map[string]any
}

// NewStateDelta creates a new empty StateDelta.
func NewStateDelta() *StateDelta {
	return &StateDelta{delta: make(map[string]any)}
}

// Set records a state change. The change is NOT applied immediately;
// it is buffered until the delta is committed.
func (s *StateDelta) Set(key string, value any) {
	s.delta[key] = value
}

// Get reads from the delta first, then falls back to the provided state map.
// This allows hooks to see their own uncommitted changes.
func (s *StateDelta) Get(key string, fallback map[string]any) (any, bool) {
	if v, ok := s.delta[key]; ok {
		return v, true
	}
	if fallback != nil {
		v, ok := fallback[key]
		return v, ok
	}
	return nil, false
}

// All returns a copy of the accumulated deltas.
func (s *StateDelta) All() map[string]any {
	out := make(map[string]any, len(s.delta))
	for k, v := range s.delta {
		out[k] = v
	}
	return out
}

// Len returns the number of accumulated changes.
func (s *StateDelta) Len() int {
	return len(s.delta)
}

// StepEffects captures side-effects produced during an agent step.
// Each step (LLM call + tool executions) can produce state changes,
// artifact updates, transfer requests, and tool approvals.
type StepEffects struct {
	// StateDelta contains key-value changes to session state.
	StateDelta map[string]any

	// ArtifactDelta tracks artifact version changes during this step.
	ArtifactDelta map[string]int64

	// TransferToAgent requests control transfer to another agent.
	TransferToAgent string

	// Escalate requests escalation (exit to parent agent).
	Escalate bool

	// ToolApprovals tracks tools awaiting human approval (HITL).
	ToolApprovals map[string]ToolApproval
}

// NewStepEffects creates a new StepEffects with initialized maps.
func NewStepEffects() *StepEffects {
	return &StepEffects{
		StateDelta:    make(map[string]any),
		ArtifactDelta: make(map[string]int64),
		ToolApprovals: make(map[string]ToolApproval),
	}
}

// ToolApproval represents a tool execution awaiting user approval.
// Used for human-in-the-loop (HITL) flows where certain tool calls
// require explicit user confirmation before proceeding.
type ToolApproval struct {
	ToolName string
	CallID   string
	Args     map[string]any
	Hint     string
	Payload  any
}

// ──────────────────────────────────────────────────────────────────────────────
// State Scoping
//
// State keys can be prefixed to control their visibility scope.
// Unprefixed keys are session-scoped (specific to the current session).
// ──────────────────────────────────────────────────────────────────────────────

// State scope prefixes for different visibility levels.
const (
	// StateScopeApp is the prefix for app-level state (shared across all users/sessions).
	StateScopeApp = "app:"

	// StateScopeUser is the prefix for user-level state (shared across sessions for a user).
	StateScopeUser = "user:"

	// StateScopeTemp is the prefix for temporary state (discarded after invocation).
	StateScopeTemp = "temp:"

	// No prefix means session-level state (specific to this session).
)
