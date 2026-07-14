package adkgo

import (
	"aicompass/foundation/logger"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	adktool "google.golang.org/adk/v2/tool"

	fagent "aicompass/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// ADK-Native Confirmation Callback
//
// Uses ADK's built-in confirmation protocol (RequestConfirmation →
// adk_request_confirmation → FunctionResponse) instead of the legacy
// io.Confirm() → ErrAuthRequired pause/resume flow.
//
// Benefits:
//   - Standard ADK protocol — no custom pause/resume mechanism
//   - Frontend receives adk_request_confirmation events natively
//   - Confirmation state tracked in session.EventActions
//   - Compatible with ADK's RequestConfirmationRequestProcessor
// ──────────────────────────────────────────────────────────────────────────────

// buildConfirmationCallback creates a BeforeToolCallback that enforces user
// confirmation using ADK's native HITL protocol.
//
// The callback:
//  1. Skips if confirmation was already provided (ctx.ToolConfirmation() != nil)
//  2. Checks per-user config via ConfirmationChecker (DB + admin policies)
//  3. If needed, calls ctx.RequestConfirmation() and returns ErrConfirmationRequired
//  4. ADK then generates adk_request_confirmation events for the frontend
//
// userID and toolSetID are captured in the closure so the callback doesn't
// need to extract them from context metadata.
func buildConfirmationCallback(
	checker ConfirmationChecker,
	agentName, userID, toolSetID string,
) llmagent.BeforeToolCallback {
	return func(ctx agent.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
		toolName := t.Name()

		// If confirmation was already provided by the user, check the result.
		if confirmation := ctx.ToolConfirmation(); confirmation != nil {
			if !confirmation.Confirmed {
				return nil, fmt.Errorf("tool %q %w", toolName, adktool.ErrConfirmationRejected)
			}
			return nil, nil // confirmed — proceed with tool execution
		}

		// No confirmation yet — check if this tool/user combination requires it.
		needsConfirmation, hint := checker.CheckConfirmation(ctx, userID, agentName, toolSetID, toolName, args)
		if !needsConfirmation {
			return nil, nil
		}

		// Build default hint if none provided.
		if hint == "" {
			hint = fmt.Sprintf("Tool '%s' requires your confirmation before executing.", toolName)
		}

		// Use ADK's native confirmation protocol.
		// This stores the request in eventActions.RequestedToolConfirmations
		// and ADK will generate an adk_request_confirmation FunctionCall event.
		if err := ctx.RequestConfirmation(hint, toolName); err != nil {
			return nil, err
		}

		// Prevent the agent from summarizing while waiting for user approval.
		ctx.Actions().SkipSummarization = true

		// Return the sentinel error — ADK's OnToolErrorCallback will handle it.
		return nil, fmt.Errorf("tool %q %w", toolName, adktool.ErrConfirmationRequired)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Native ADK-Go Tool Callbacks
//
// These functions produce native ADK BeforeToolCallback / AfterToolCallback
// values. Using native callbacks avoids the middleware-bridge indirection
// (sync.Map for phase persistence, phase constants, double-pass chain) and
// gives direct access to ADK's FunctionCallID for accurate per-call timing.
// ──────────────────────────────────────────────────────────────────────────────

// toolLoggingCallbacks returns a pair of native ADK callbacks that log every
// tool call with its arguments, result, and wall-clock duration.
//
// The before callback records the start time keyed by FunctionCallID.
// The after callback retrieves the start time to compute the exact duration.
func toolLoggingCallbacks(
	log *logger.Logger,
	agentName, sessionID, userID string,
) (llmagent.BeforeToolCallback, llmagent.AfterToolCallback) {
	var startTimes sync.Map // FunctionCallID → time.Time

	before := func(ctx agent.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
		startTimes.Store(ctx.FunctionCallID(), time.Now())
		log.Info(ctx, "tool call started",
			"agent", agentName,
			"thread_id", sessionID,
			"user_id", userID,
			"tool", t.Name(),
			"arguments", args,
		)
		return nil, nil
	}

	after := func(ctx agent.Context, t adktool.Tool, _ map[string]any, result map[string]any, toolErr error) (map[string]any, error) {
		start := time.Now()
		if v, ok := startTimes.LoadAndDelete(ctx.FunctionCallID()); ok {
			start = v.(time.Time)
		}
		duration := time.Since(start)

		if toolErr != nil {
			log.Error(ctx, "tool call failed",
				"agent", agentName,
				"thread_id", sessionID,
				"tool", t.Name(),
				"duration", duration,
				"error", toolErr,
			)
		} else {
			log.Info(ctx, "tool call completed",
				"agent", agentName,
				"thread_id", sessionID,
				"tool", t.Name(),
				"duration", duration,
				"result", result,
			)
		}
		return nil, nil
	}

	return before, after
}

// buildBeforeOnlyCallbacks converts before-only foundation middlewares to a
// single native ADK-Go BeforeToolCallback. This is used for enforcement
// middlewares that only need to inspect or block tool execution:
//
//   - BlockMiddleware (blocks tools the user has disabled)
//
// The middleware chain runs with a no-op terminal — if any middleware returns
// an error (e.g. tool blocked), it is surfaced as an error result map so
// ADK-Go short-circuits tool execution and shows the reason to the LLM.
//
// Returns nil if no non-nil middlewares are provided.
func buildBeforeOnlyCallbacks(
	agentName, sessionID, userID string,
	middlewares ...fagent.Middleware,
) []llmagent.BeforeToolCallback {
	var mws []fagent.Middleware
	for _, m := range middlewares {
		if m != nil {
			mws = append(mws, m)
		}
	}
	if len(mws) == 0 {
		return nil
	}

	return []llmagent.BeforeToolCallback{
		func(ctx agent.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
			fCtx := newEnforcerContext(ctx, t, args, agentName, sessionID, userID)
			chain := fagent.Chain(mws...)
			if err := chain(fagent.HandlerFunc(func(*fagent.Context) error { return nil })).Handle(fCtx); err != nil {
				return map[string]any{errorKey: err.Error()}, nil
			}
			return nil, nil
		},
	}
}

// newEnforcerContext creates a foundation middleware Context from an ADK
// tool.Context for use in before-only enforcement callbacks (block, confirm).
func newEnforcerContext(
	ctx context.Context,
	t adktool.Tool,
	args map[string]any,
	agentName, sessionID, userID string,
) *fagent.Context {
	bridge := &adkToolBridge{name: t.Name(), desc: t.Description()}
	fCtx := fagent.NewToolContext(ctx, bridge, args)
	fCtx.Metadata["agent_name"] = agentName
	fCtx.Metadata["thread_id"] = sessionID
	fCtx.Metadata["user_id"] = userID
	return fCtx
}

// adkToolBridge adapts an ADK tool name/description to the foundation Tool
// interface so foundation middlewares can inspect the tool being called.
type adkToolBridge struct {
	name string
	desc string
}

func (t *adkToolBridge) Name() string           { return t.name }
func (t *adkToolBridge) Description() string    { return t.desc }
func (t *adkToolBridge) Schema() *fagent.Schema { return nil }

// ──────────────────────────────────────────────────────────────────────────────
// Model Logging Callbacks
//
// modelLoggingCallbacks returns a pair of native ADK-Go model callbacks that
// log every model invocation with token usage and wall-clock duration.
//
// The before callback records the start time keyed by invocation ID.
// The after callback logs the full model response including token counts,
// making it easy to correlate latency and cost with specific agent calls.
// ──────────────────────────────────────────────────────────────────────────────

// modelLoggingCallbacks returns Before/After model callbacks for timing + token
// usage logging. The AfterModelCallback is the ideal place to log this data
// because the final token counts are only available in the full model response.
func modelLoggingCallbacks(
	log *logger.Logger,
	agentName, sessionID string,
) (llmagent.BeforeModelCallback, llmagent.AfterModelCallback) {
	var startTimes sync.Map // invocationID → time.Time

	before := func(ctx agent.Context, req *model.LLMRequest) (*model.LLMResponse, error) {
		startTimes.Store(ctx.InvocationID(), time.Now())
		log.Info(ctx, "model call started",
			"agent", agentName,
			"thread_id", sessionID,
			"tool_count", len(req.Tools),
		)
		return nil, nil // nil = proceed with the actual model call
	}

	after := func(ctx agent.Context, resp *model.LLMResponse, respErr error) (*model.LLMResponse, error) {
		start := time.Now()
		if v, ok := startTimes.LoadAndDelete(ctx.InvocationID()); ok {
			start = v.(time.Time)
		}
		duration := time.Since(start)

		if respErr != nil {
			log.Error(ctx, "model call failed",
				"agent", agentName,
				"thread_id", sessionID,
				"duration", duration,
				"error", respErr,
			)
			return nil, nil // nil = propagate the original error
		}

		if resp != nil && resp.UsageMetadata != nil {
			u := resp.UsageMetadata
			log.Info(ctx, "model call completed",
				"agent", agentName,
				"thread_id", sessionID,
				"duration", duration,
				"prompt_tokens", u.PromptTokenCount,
				"completion_tokens", u.CandidatesTokenCount,
				"total_tokens", u.TotalTokenCount,
			)
		} else {
			log.Info(ctx, "model call completed",
				"agent", agentName,
				"thread_id", sessionID,
				"duration", duration,
			)
		}
		return nil, nil // nil = use the original response unchanged
	}

	return before, after
}

// ──────────────────────────────────────────────────────────────────────────────
// OnToolErrorCallback
//
// ADK-Go v1.0.0 added ErrConfirmationRequired / ErrConfirmationRejected as
// exported sentinel errors so callers can detect them via errors.Is().
// Previously the retry-and-reflect plugin matched them by string; now it uses
// errors.Is(), which is more robust.
//
// toolErrorCallback returns an llmagent.OnToolErrorCallback that:
//  1. Converts ErrConfirmationRequired/ErrConfirmationRejected to clean
//     result maps (HITL flow-control, not failures — the LLM should not
//     see a raw error string for these).
//  2. Logs unexpected native tool errors (tool-not-found, exitlooptool
//     failures, etc.) that our wrapped tools would never produce.
//
// This callback fires only when a Go error is returned from BeforeToolCallback
// (if it returns (nil, err)) or from tool.Run() — not from our own wrapped
// tools, which surface errors as {"error":"..."} maps and return (map, nil).
// ──────────────────────────────────────────────────────────────────────────────

func toolErrorCallback(
	log *logger.Logger,
	agentName, sessionID string,
) llmagent.OnToolErrorCallback {
	return func(ctx agent.Context, t adktool.Tool, args map[string]any, err error) (map[string]any, error) {
		// ── Confirmation-flow errors (ADK-Go v1.0.0 sentinels) ──────────────
		// These are HITL flow-control signals, not actual failures.
		// Return a structured result so the LLM gets clear guidance instead
		// of a raw sentinel error string.
		if errors.Is(err, adktool.ErrConfirmationRequired) {
			if log != nil {
				log.Info(ctx, "tool awaiting confirmation",
					"agent", agentName,
					"thread_id", sessionID,
					"tool", t.Name(),
				)
			}
			return map[string]any{
				"status":  "confirmation_required",
				"message": fmt.Sprintf("Tool '%s' requires user confirmation before proceeding.", t.Name()),
			}, nil
		}
		if errors.Is(err, adktool.ErrConfirmationRejected) {
			if log != nil {
				log.Info(ctx, "tool confirmation rejected",
					"agent", agentName,
					"thread_id", sessionID,
					"tool", t.Name(),
				)
			}
			return map[string]any{
				"status":  "rejected",
				"message": fmt.Sprintf("Tool '%s' execution was rejected by the user.", t.Name()),
			}, nil
		}

		// ── Unexpected native tool error ─────────────────────────────────────
		// Log it and return nil so ADK-Go converts the error to {"error":"..."}.
		if log != nil {
			log.Error(ctx, "native tool error",
				"agent", agentName,
				"thread_id", sessionID,
				"tool", t.Name(),
				"error", err,
			)
		}
		return nil, nil
	}
}
