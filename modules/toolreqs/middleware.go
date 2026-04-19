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

// toolMembershipChecker answers "is tool X a member of tool set Y?".
// Implemented by [Service] via the tool registry.
type toolMembershipChecker interface {
	IsToolInToolSet(ctx context.Context, toolSetID, toolName string) (bool, error)
}

// CheckConfirmation determines whether a tool call needs user
// confirmation, consulting per-user DB config in three layers:
//
//  1. Individual tool config for (user, agent, toolSetID, toolName).
//  2. Tool-set level config for (user, agent, toolSetID) when known.
//  3. Fallback scan: when toolSetID is unknown, walk every tool-set that
//     the user flagged require_confirmation and ask the membership checker
//     whether this tool belongs to it.
//
// Returns (needsConfirmation, hint). Callers can pass a nil store to
// short-circuit to "no confirmation" — useful for tests that don't wire
// persistence.
func CheckConfirmation(
	ctx context.Context,
	store *PermissionStore,
	membership toolMembershipChecker,
	userID, agentID, toolSetID, toolName string,
	_ map[string]any,
) (bool, string) {
	if userID == "" || store == nil {
		return false, ""
	}

	if ok, hint := toolLevelConfirmation(ctx, store, userID, agentID, toolSetID, toolName); ok {
		return true, hint
	}

	if toolSetID != "" {
		return toolSetLevelConfirmation(ctx, store, userID, agentID, toolSetID)
	}

	return fallbackToolSetScan(ctx, store, membership, userID, agentID, toolName)
}

// requiresConfirmation is true when the config's action or legacy flag asks
// for user approval. Captures the dual-signal invariant in one place.
func requiresConfirmation(cfg ToolRequirementConfig) bool {
	return cfg.Action == RequirementActionRequireConfirmation || cfg.RequireConfirmation
}

func toolLevelConfirmation(ctx context.Context, store *PermissionStore, userID, agentID, toolSetID, toolName string) (bool, string) {
	cfg, err := store.GetToolRequirementConfig(ctx, userID, agentID, toolSetID, toolName)
	if err != nil || !requiresConfirmation(cfg) {
		return false, ""
	}
	return true, cfg.ConfirmationHint
}

func toolSetLevelConfirmation(ctx context.Context, store *PermissionStore, userID, agentID, toolSetID string) (bool, string) {
	cfg, err := store.GetToolSetRequirementConfig(ctx, userID, agentID, toolSetID)
	if err != nil || !requiresConfirmation(cfg) {
		return false, ""
	}
	return true, cfg.ConfirmationHint
}

// fallbackToolSetScan walks every confirmation-flagged tool set the user owns
// and asks the membership checker whether the tool belongs there. Used only
// when the caller could not identify the tool set up-front.
func fallbackToolSetScan(ctx context.Context, store *PermissionStore, membership toolMembershipChecker, userID, agentID, toolName string) (bool, string) {
	if membership == nil {
		return false, ""
	}
	confToolSets, err := store.ToolSetsWithConfirmation(ctx, userID, agentID)
	if err != nil || len(confToolSets) == 0 {
		return false, ""
	}
	for _, tsID := range confToolSets {
		cfg, err := store.GetToolSetRequirementConfig(ctx, userID, agentID, tsID)
		if err != nil || !requiresConfirmation(cfg) {
			continue
		}
		inSet, lookupErr := membership.IsToolInToolSet(ctx, tsID, toolName)
		if lookupErr == nil && inSet {
			return true, cfg.ConfirmationHint
		}
		// Membership lookup failed or returned false — fail-open so unknown
		// custom tools don't gratuitously prompt.
	}
	return false, ""
}
