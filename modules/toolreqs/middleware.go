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

// ConfirmationConfigResolver reads per-user, per-agent tool requirement configs
// from persistent storage (DB + cache). Implemented by [Service].
type ConfirmationConfigResolver interface {
	GetToolRequirementConfig(ctx context.Context, userID, agentID, toolSetID, toolID string) (ToolRequirementConfig, error)
}

// CheckConfirmation determines whether a tool call needs user confirmation,
// checking both per-user DB config and admin-level global policies.
//
// Returns (needsConfirmation, hint). The hint is shown to the user in the
// confirmation dialog. If empty, callers should generate a default hint.
//
// This is used by the adapter layer to build ADK-native BeforeToolCallbacks
// that call ctx.RequestConfirmation() instead of the legacy io.Confirm() flow.
func CheckConfirmation(
	ctx context.Context,
	resolver ConfirmationConfigResolver,
	cr *ConfirmationRequirement,
	userID, agentID, toolSetID, toolName string,
	args map[string]any,
) (bool, string) {
	// 1. Check per-user requirement config from DB (primary source).
	if userID != "" && resolver != nil {
		cfg, err := resolver.GetToolRequirementConfig(ctx, userID, agentID, toolSetID, toolName)
		if err == nil && (cfg.Action == RequirementActionRequireConfirmation || cfg.RequireConfirmation) {
			return true, cfg.ConfirmationHint
		}
	}

	// 2. Check global/admin in-memory policies (override layer).
	if cr != nil && cr.RequiresConfirmation(agentID, toolName, args) {
		return true, cr.getHint(agentID, toolName)
	}

	return false, ""
}
