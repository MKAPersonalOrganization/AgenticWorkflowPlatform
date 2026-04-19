package toolreqs

import (
	"context"
	"fmt"

	"aicompass/foundation/agent"
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
			if !c.IsTool() {
				return next.Handle(c)
			}

			toolName := ""
			if c.Tool != nil && c.Tool.Tool != nil {
				toolName = c.Tool.Tool.Name()
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
// Confirmation — per-user runtime check for ADK native HITL
// =============================================================================
//
// Two narrow collaborators (instead of one kitchen-sink
// ConfirmationConfigResolver) so the CheckConfirmation flow states its
// dependencies plainly:
//
//   - a *PermissionStore for DB-backed per-user config reads
//   - a toolMembershipChecker for "is this tool in that set?" lookups
//
// The store is a concrete type because there's only one real
// implementation; the membership lookup is a small interface so that
// tests and other checkers (e.g. per-user membership in the future)
// can substitute without dragging in the persistence layer.

// toolMembershipChecker answers "is tool X a member of tool set Y?".
// Implemented by [Service] via the tool registry.
type toolMembershipChecker interface {
	IsToolInToolSet(ctx context.Context, toolSetID, toolName string) (bool, error)
}

// CheckConfirmation determines whether a tool call needs user
// confirmation, consulting both per-user DB config (the store) and
// admin-level global policies (the ConfirmationRequirement). Returns
// (needsConfirmation, hint).
//
// Callers inside the package can pass a nil store to test only the
// global-policy path; production callers go through
// [Service.CheckConfirmation] which wires both collaborators.
func CheckConfirmation(
	ctx context.Context,
	store *PermissionStore,
	membership toolMembershipChecker,
	cr *ConfirmationRequirement,
	userID, agentID, toolSetID, toolName string,
	args map[string]any,
) (bool, string) {
	// 1. Check per-user individual tool config from DB (primary source).
	if userID != "" && store != nil {
		cfg, err := store.GetToolRequirementConfig(ctx, userID, agentID, toolSetID, toolName)
		if err == nil && (cfg.Action == RequirementActionRequireConfirmation || cfg.RequireConfirmation) {
			return true, cfg.ConfirmationHint
		}

		// 1b. Check tool-set level config if we know the tool-set ID.
		if toolSetID != "" {
			tsCfg, err := store.GetToolSetRequirementConfig(ctx, userID, agentID, toolSetID)
			if err == nil && (tsCfg.Action == RequirementActionRequireConfirmation || tsCfg.RequireConfirmation) {
				return true, tsCfg.ConfirmationHint
			}
		} else if membership != nil {
			// 1c. No tool-set ID provided — scan every tool-set flagged with
			// require_confirmation and ask the membership checker whether
			// the tool belongs. Preferred path is 1b (toolSetID known); this
			// branch is the fallback for custom-tool invocations.
			confToolSets, err := store.ToolSetsWithConfirmation(ctx, userID, agentID)
			if err == nil && len(confToolSets) > 0 {
				for _, tsID := range confToolSets {
					tsCfg, err := store.GetToolSetRequirementConfig(ctx, userID, agentID, tsID)
					if err == nil && (tsCfg.Action == RequirementActionRequireConfirmation || tsCfg.RequireConfirmation) {
						toolInSet, lookupErr := membership.IsToolInToolSet(ctx, tsID, toolName)
						if lookupErr == nil && toolInSet {
							return true, tsCfg.ConfirmationHint
						}
						// Membership lookup failed / returned false — fail-open
						// so unknown custom tools don't gratuitously prompt.
					}
				}
			}
		}
	}

	// 2. Check global/admin in-memory policies (override layer).
	if cr != nil && cr.RequiresConfirmation(agentID, toolName, args) {
		return true, cr.getHint(agentID, toolName)
	}

	return false, ""
}
