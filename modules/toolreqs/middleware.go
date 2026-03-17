package toolreqs

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agent/io"
)

// =============================================================================
// Block Middleware — defense-in-depth enforcement at execution time
// =============================================================================

// blockChecker is the narrow interface needed by [BlockMiddleware].
// It is implemented by [Service].
type blockChecker interface {
	HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool
}

// BlockMiddleware returns a [agent.Middleware] that prevents execution of tools
// the user has blocked via the requirement configuration.
//
// This is a safety net: normally, blocked tools are filtered out during tool
// resolution (resolveTools) so the LLM never sees them. However, race
// conditions (config updated mid-conversation) or bugs in resolution could
// allow a blocked tool through. This middleware catches those cases.
//
// The middleware extracts user_id from context metadata (populated by the
// middlewareBridge) and calls HasPermission to verify the tool is still allowed.
func BlockMiddleware(checker blockChecker, agentID string) agent.Middleware {
	return func(next agent.Handler) agent.Handler {
		return agent.HandlerFunc(func(c *agent.Context) error {
			if !c.IsFunction() {
				return next.Handle(c)
			}

			toolName := ""
			if c.Function != nil && c.Function.Tool != nil {
				toolName = c.Function.Tool.Name()
			}
			if toolName == "" {
				return next.Handle(c)
			}

			userID, _ := c.Metadata["user_id"].(string)
			toolSetID, _ := c.Metadata["tool_set_id"].(string)

			if userID != "" && checker != nil {
				if !checker.HasPermission(c.Context, userID, agentID, toolSetID, toolName) {
					c.Result = map[string]any{
						"result":  fmt.Sprintf("Tool '%s' is blocked by your configuration.", toolName),
						"blocked": true,
						"tool":    toolName,
					}
					return nil
				}
			}

			return next.Handle(c)
		})
	}
}

// =============================================================================
// Confirmation Middleware — per-user runtime enforcement
// =============================================================================

// ConfirmationConfigResolver reads per-user, per-agent tool requirement configs
// from persistent storage (DB + cache). Implemented by [Service].
type ConfirmationConfigResolver interface {
	GetToolRequirementConfig(ctx context.Context, userID, agentID, toolSetID, toolID string) (ToolRequirementConfig, error)
}

// ConfirmationMiddleware returns a [agent.Middleware] that enforces user
// confirmation for tools whose per-user requirement config has action
// [RequirementActionRequireConfirmation].
//
// This middleware reads the user's config from the database (via the resolver)
// at execution time, ensuring it respects per-user, per-agent isolation.
// It also checks the in-memory [ConfirmationRequirement] for admin-level
// global policies that override individual user configs.
//
// At execution time, the middleware:
//
//  1. Extracts user_id from context metadata (set by middlewareBridge).
//  2. Checks the per-user requirement config from the DB via the resolver.
//  3. Also checks global/admin policies via [ConfirmationRequirement].
//  4. If either requires confirmation, calls [io.Confirm] to trigger the
//     pause/resume flow (PauseHandler in web, StdinHandler in CLI).
//  5. If the user denies, sets a denial result on the context.
//  6. If the user approves, proceeds with normal execution.
func ConfirmationMiddleware(resolver ConfirmationConfigResolver, cr *ConfirmationRequirement, agentID string) agent.Middleware {
	return func(next agent.Handler) agent.Handler {
		return agent.HandlerFunc(func(c *agent.Context) error {
			if !c.IsFunction() {
				return next.Handle(c)
			}

			toolName := ""
			if c.Function != nil && c.Function.Tool != nil {
				toolName = c.Function.Tool.Name()
			}
			if toolName == "" {
				return next.Handle(c)
			}

			var args map[string]any
			if c.Function != nil {
				args = c.Function.Arguments
			}

			userID, _ := c.Metadata["user_id"].(string)
			toolSetID, _ := c.Metadata["tool_set_id"].(string)

			needsConfirmation := false
			hint := ""

			// 1. Check per-user requirement config from DB (primary source).
			if userID != "" && resolver != nil {
				cfg, err := resolver.GetToolRequirementConfig(c.Context, userID, agentID, toolSetID, toolName)
				if err == nil && (cfg.Action == RequirementActionRequireConfirmation || cfg.RequireConfirmation) {
					needsConfirmation = true
					hint = cfg.ConfirmationHint
				}
			}

			// 2. Check global/admin in-memory policies (override layer).
			if !needsConfirmation && cr != nil && cr.RequiresConfirmation(agentID, toolName, args) {
				needsConfirmation = true
				hint = cr.getHint(agentID, toolName)
			}

			if !needsConfirmation {
				return next.Handle(c)
			}

			// Build default hint if none provided.
			if hint == "" {
				hint = fmt.Sprintf("Tool '%s' requires your confirmation before executing.", toolName)
			}

			// Trigger pause/resume flow via IO handler:
			//   - PauseHandler (web): returns ErrAuthRequired → engine pauses
			//   - StdinHandler (CLI): blocks on stdin → user types y/n
			//   - NoopHandler (test): auto-approves
			approved, err := io.Confirm(c.Context, hint, toolName)
			if err != nil {
				return err
			}

			if !approved {
				c.Result = map[string]any{
					"result": fmt.Sprintf("Tool '%s' execution was denied by the user.", toolName),
					"denied": true,
					"tool":   toolName,
				}
				return nil
			}

			return next.Handle(c)
		})
	}
}
