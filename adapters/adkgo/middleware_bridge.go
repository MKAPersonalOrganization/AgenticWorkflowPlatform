package adkgo

import (
	"context"
	"sync"

	"google.golang.org/adk/agent/llmagent"
	adktool "google.golang.org/adk/tool"

	fagent "school-with-ai/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// Middleware Bridge
//
// ADK-Go splits tool lifecycle into BeforeToolCallback and AfterToolCallback.
// The foundation middleware system models this as a single linear chain
// (before → execute → after, like HTTP middleware). We bridge these two models
// using a shared-context approach:
//
//   1. BeforeToolCallback: creates a foundation Context with phase="before",
//      runs the chain (giving middlewares their "before" phase), then stores
//      the context in a sync.Map keyed by the function call ID.
//
//   2. AfterToolCallback: retrieves the stored context, updates the phase to
//      "after" and populates c.Result, then runs the chain again (giving
//      middlewares their "after" phase).
//
// Phase-aware middlewares (e.g. ToolLogging) read c.Metadata[ToolPhaseKey] to
// determine which half they are in, avoiding double-logging and enabling
// accurate duration measurement across the callback split.
//
// If BeforeToolCallback returns an error result (tool blocked by middleware),
// AfterToolCallback is still called by ADK-Go — the bridge handles this
// correctly by retrieving the stored context and running the after phase.
// ──────────────────────────────────────────────────────────────────────────────

// middlewareBridge wraps the foundation FunctionMiddleware chain into ADK-Go's
// split before/after tool callback model.
type middlewareBridge struct {
	middlewares []fagent.Middleware
	agentName   string
	sessionID   string
	userID      string

	// pending stores the shared middleware Context between the before and after
	// phases, keyed by the ADK-Go function call ID.
	pending sync.Map
}

// newMiddlewareBridge collects all function middlewares from the agent config
// (both explicit and requirement-provided) and returns a bridge, or nil if
// there are no middlewares to run.
//
// Additional middlewares (e.g., from the toolreqs confirmation system) can be
// prepended via the extraMiddlewares parameter.
func newMiddlewareBridge(agentCfg *fagent.Config, agentName, sessionID, userID string, extraMiddlewares ...fagent.Middleware) *middlewareBridge {
	var mws []fagent.Middleware

	// 1. Extra middlewares (e.g., toolreqs confirmation) run first.
	mws = append(mws, extraMiddlewares...)

	if agentCfg != nil {
		// 2. Explicit function middlewares from agent config.
		mws = append(mws, agentCfg.FunctionMiddlewares...)

		// 3. Requirement-provided middlewares.
		for _, req := range agentCfg.Requirements {
			if fmp, ok := req.(fagent.FunctionMiddlewareProvider); ok {
				mws = append(mws, fmp.FunctionMiddleware())
			}
		}
	}

	if len(mws) == 0 {
		return nil
	}

	return &middlewareBridge{
		middlewares: mws,
		agentName:   agentName,
		sessionID:   sessionID,
		userID:      userID,
	}
}

// newContext creates a foundation middleware Context with properly populated
// metadata, matching what middleware_common.go expects.
func (mb *middlewareBridge) newContext(ctx context.Context, toolName, toolDesc string, args map[string]any) *fagent.Context {
	tool := &adkToolBridge{name: toolName, desc: toolDesc}
	fCtx := fagent.NewToolContext(ctx, tool, args)
	fCtx.Metadata["agent_name"] = mb.agentName
	fCtx.Metadata["thread_id"] = mb.sessionID
	fCtx.Metadata["message_id"] = "" // Not available at tool-call level.
	fCtx.Metadata["user_id"] = mb.userID
	return fCtx
}

// runChain executes the middleware chain with the given context and terminal handler.
func (mb *middlewareBridge) runChain(fCtx *fagent.Context, terminal fagent.HandlerFunc) error {
	chain := fagent.Chain(mb.middlewares...)
	return chain(terminal).Handle(fCtx)
}

// beforeToolCallbacks returns the ADK-Go BeforeToolCallback slice.
//
// The callback:
//  1. Creates a shared foundation Context with phase="before".
//  2. Stores it in mb.pending keyed by the function call ID.
//  3. Runs the middleware chain — middlewares see the "before" phase.
//  4. If the chain returns an error, the error is surfaced as a tool result map
//     (so ADK-Go short-circuits tool execution but still calls AfterToolCallback).
func (mb *middlewareBridge) beforeToolCallbacks() []llmagent.BeforeToolCallback {
	return []llmagent.BeforeToolCallback{
		func(ctx adktool.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
			fCtx := mb.newContext(ctx, t.Name(), t.Description(), args)
			fCtx.Metadata[fagent.ToolPhaseKey] = fagent.ToolPhaseBefore

			// Store for the after phase before running the chain, so it is
			// always available even if the chain returns early.
			callID := ctx.FunctionCallID()
			mb.pending.Store(callID, fCtx)

			if err := mb.runChain(fCtx, func(c *fagent.Context) error {
				return nil // Terminal: allow tool to proceed.
			}); err != nil {
				return map[string]any{errorKey: err.Error()}, nil
			}

			return nil, nil // nil = continue with original args.
		},
	}
}

// afterToolCallbacks returns the ADK-Go AfterToolCallback slice.
//
// The callback:
//  1. Retrieves the shared Context stored by beforeToolCallbacks.
//  2. Updates the phase to "after" and populates c.Result.
//  3. Runs the middleware chain — middlewares see the "after" phase with the result.
//  4. Always best-effort: errors from post-execution middlewares are discarded.
func (mb *middlewareBridge) afterToolCallbacks() []llmagent.AfterToolCallback {
	return []llmagent.AfterToolCallback{
		func(ctx adktool.Context, t adktool.Tool, args, result map[string]any, _ error) (map[string]any, error) {
			callID := ctx.FunctionCallID()

			var fCtx *fagent.Context
			if stored, ok := mb.pending.LoadAndDelete(callID); ok {
				fCtx = stored.(*fagent.Context)
			} else {
				// No stored context (e.g. bridge was bypassed) — create fresh one.
				fCtx = mb.newContext(ctx, t.Name(), t.Description(), args)
			}

			fCtx.Metadata[fagent.ToolPhaseKey] = fagent.ToolPhaseAfter
			fCtx.Result = result

			// Best-effort: don't fail the tool over post-execution middleware errors.
			_ = mb.runChain(fCtx, func(c *fagent.Context) error {
				return nil
			})

			return nil, nil // nil = keep original result unchanged.
		},
	}
}

// adkToolBridge adapts an adk-go tool name/description to the foundation Tool interface.
type adkToolBridge struct {
	name string
	desc string
}

func (t *adkToolBridge) Name() string           { return t.name }
func (t *adkToolBridge) Description() string    { return t.desc }
func (t *adkToolBridge) Schema() *fagent.Schema { return nil }
