package toolreqs

import (
	"context"
	"fmt"
)

// =============================================================================
// permissionChecker — internal interface used by PermissionRequirement
// =============================================================================

// permissionChecker is the internal interface that PermissionRequirement uses
// to access cached permission data. It is implemented by [Service].
//
// This avoids circular construction: Service creates PermissionRequirement,
// then sets itself as the checker via setChecker.
//
// All methods evaluate the requirement config action (allow/block/require_confirmation)
// rather than a boolean Enabled flag.
type permissionChecker interface {
	// isToolSetAllowed checks whether a tool set is not blocked.
	isToolSetAllowed(ctx context.Context, userID, agentID, toolSetID string) bool
	// isToolAllowed checks whether a specific tool is not blocked.
	// Falls back to tool-set level when no explicit individual config exists.
	isToolAllowed(ctx context.Context, userID, agentID, toolSetID, toolID string) bool
	// getAllowedToolSets returns all tool set IDs whose config action is not "block".
	getAllowedToolSets(ctx context.Context, userID, agentID string) ([]string, error)
}

// =============================================================================
// PermissionRequirement — evaluates tool requirement configs
// =============================================================================

// PermissionRequirement checks whether a tool or tool set is allowed (not
// blocked) for the given user/agent based on its requirement config action.
// It delegates to the [Service]'s cached permission lookups (backed by
// PostgreSQL + Redis).
type PermissionRequirement struct {
	checker permissionChecker
}

// newPermissionRequirement creates a permission requirement.
// The checker is set after construction via setChecker (to break the init cycle).
func newPermissionRequirement() *PermissionRequirement {
	return &PermissionRequirement{}
}

// setChecker wires the permission checker (the owning Service) after construction.
func (r *PermissionRequirement) setChecker(c permissionChecker) {
	r.checker = c
}

// Kind implements [ToolRequirement].
func (r *PermissionRequirement) Kind() RequirementKind { return RequirementPermission }

// Evaluate implements [ToolRequirement].
//
// Evaluation logic:
//  1. If no checker is configured → allow (fail-open for tests/dev).
//  2. If both ToolSetID and ToolID are set → check individual tool config
//     (falls back to tool-set when no explicit override exists).
//  3. If only ToolSetID is set (no ToolID) → check tool-set config.
//  4. If the config action is "block" → block with reason.
func (r *PermissionRequirement) Evaluate(ctx context.Context, req *ToolEvalRequest) (*RequirementResult, error) {
	if r.checker == nil {
		return &RequirementResult{Satisfied: true, Action: ActionAllow}, nil
	}

	// When both tool set and tool are specified, check individual tool config
	// (which internally falls back to tool set when no explicit override exists).
	if req.ToolSetID != "" && req.ToolID != "" {
		if !r.checker.isToolAllowed(ctx, req.UserID, req.AgentID, req.ToolSetID, req.ToolID) {
			return &RequirementResult{
				Satisfied: false,
				Action:    ActionBlock,
				Reason:    fmt.Sprintf("tool %q in set %q is blocked", req.ToolID, req.ToolSetID),
			}, nil
		}
		return &RequirementResult{Satisfied: true, Action: ActionAllow}, nil
	}

	// Tool-set only check (no individual tool).
	if req.ToolSetID != "" {
		if !r.checker.isToolSetAllowed(ctx, req.UserID, req.AgentID, req.ToolSetID) {
			return &RequirementResult{
				Satisfied: false,
				Action:    ActionBlock,
				Reason:    fmt.Sprintf("tool set %q is blocked for this agent", req.ToolSetID),
			}, nil
		}
	}

	return &RequirementResult{Satisfied: true, Action: ActionAllow}, nil
}

// GetAllowedToolSets is a convenience method that delegates directly to
// the cached permission lookups. Used by the [Service] for tool resolution.
func (r *PermissionRequirement) GetAllowedToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	if r.checker == nil {
		return nil, nil
	}
	return r.checker.getAllowedToolSets(ctx, userID, agentID)
}

// Verify interface compliance.
var _ ToolRequirement = (*PermissionRequirement)(nil)
