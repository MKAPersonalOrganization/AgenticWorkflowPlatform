package agent

import "context"

// ──────────────────────────────────────────────────────────────────────────────
// Hook types — lifecycle interception points for agent execution.
//
// Hooks complement Middleware with a simpler interception model. Where
// Middleware wraps execution (before → handler → after), hooks run at
// discrete lifecycle points and can short-circuit by returning a result.
//
// Hooks are evaluated sequentially. If any hook returns non-nil content
// (Event/result) or a non-nil error, subsequent hooks AND the actual
// operation are skipped (early-exit semantics). Return (nil, nil) to
// continue normally.
//
// Three interception levels:
//   - Run hooks:   around the entire agent run
//   - Tool hooks:  around each tool execution
//   - Model hooks: around each LLM call
// ──────────────────────────────────────────────────────────────────────────────

// HookContext provides read-write access to invocation state during
// hook execution. Hooks can read session state, buffer state changes
// via a StateDelta, and access artifacts.
type HookContext struct {
	context.Context

	// Invocation provides access to the current invocation.
	Invocation *Invocation

	// AgentName is the name of the agent being executed.
	AgentName string

	// SessionID is the current session/thread ID.
	SessionID string

	// UserID is the current user ID.
	UserID string

	// State provides read-write access to session state.
	// Writes are accumulated as a delta and applied when committed.
	State *StateDelta

	// Artifacts provides access to artifact storage.
	Artifacts Artifacts
}

// ── Run Hooks ───────────────────────────────────────────────────────────────

// BeforeRunHook is called before the agent starts its run.
// If it returns non-nil Event or error, the agent run is skipped.
// Return (nil, nil) to continue with normal execution.
type BeforeRunHook func(ctx *HookContext) (Event, error)

// AfterRunHook is called after the agent completes its run.
// If it returns non-nil Event or error, a new event is emitted.
type AfterRunHook func(ctx *HookContext) (Event, error)

// ── Tool Hooks ──────────────────────────────────────────────────────────────

// BeforeToolHook is called before each tool execution.
// If it returns non-nil result, the tool execution is skipped and
// the returned map is used as the tool result.
type BeforeToolHook func(ctx *HookContext, tool Tool, args map[string]any) (map[string]any, error)

// AfterToolHook is called after each tool execution.
// If it returns non-nil result, the returned map replaces the original result.
type AfterToolHook func(ctx *HookContext, tool Tool, args, result map[string]any, err error) (map[string]any, error)

// ── Model Hooks ─────────────────────────────────────────────────────────────

// BeforeModelHook is called before each LLM call.
// If it returns non-nil GenerateResponse, the LLM call is skipped.
type BeforeModelHook func(ctx *HookContext, req *GenerateRequest) (*GenerateResponse, error)

// AfterModelHook is called after each LLM call.
// If it returns non-nil GenerateResponse, it replaces the original response.
type AfterModelHook func(ctx *HookContext, resp *GenerateResponse, err error) (*GenerateResponse, error)
