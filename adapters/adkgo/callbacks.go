package adkgo

import (
	"context"
	"sync"
	"time"

	"google.golang.org/adk/agent/llmagent"
	adktool "google.golang.org/adk/tool"

	fagent "school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
)

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

	before := func(ctx adktool.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
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

	after := func(ctx adktool.Context, t adktool.Tool, _ map[string]any, result map[string]any, toolErr error) (map[string]any, error) {
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
//   - ConfirmationMiddleware (requires user approval before proceeding)
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
		func(ctx adktool.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
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
