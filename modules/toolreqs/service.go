package toolreqs

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"context"
)

// Service is the runtime decision-making surface for tool requirements:
// "given this user/agent/tool, should we allow it? does it need a
// confirmation? should the block middleware be installed?". It does NOT
// own persistence — that's [PermissionStore]'s job.
//
// The split follows the cohesion principle:
//
//   - PermissionStore keeps the 15 related CRUD/query methods together
//     (they all read and write the same rows and touch the same caches).
//   - Service keeps the runtime decisions: permission checks, confirmation
//     lookups, and the block middleware factory.
//
// Callers take whichever surface they actually use:
//
//   - adkgo adapter (permission + confirmation + middleware) → *Service
//   - chat HTTP handlers (config CRUD)                       → *PermissionStore
type Service struct {
	log          *logger.Logger
	store        *PermissionStore
	toolRegistry *toolregistry.Registry
}

// ServiceConfig contains dependencies for creating a [Service].
type ServiceConfig struct {
	Log          *logger.Logger
	Store        *PermissionStore
	ToolRegistry *toolregistry.Registry // for IsToolInToolSet lookups
}

// NewService creates a new tool requirements service.
func NewService(cfg ServiceConfig) *Service {
	return &Service{
		log:          cfg.Log,
		store:        cfg.Store,
		toolRegistry: cfg.ToolRegistry,
	}
}

// GetEnabledToolSets returns the tool set IDs that are allowed for the
// given user/agent. Primary entry point used during tool resolution.
//
// Accepts plain strings because this satisfies adkgo.ToolRequirementsService
// (a neutral port); internal call sites re-impose the typed IDs so the
// positional-swap guardrails survive downstream.
func (s *Service) GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	return s.store.AllowedToolSets(ctx, UserID(userID), AgentID(agentID))
}

// HasToolSetPermission reports whether a tool set is not blocked for
// this user/agent.
func (s *Service) HasToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) bool {
	return s.store.IsToolSetAllowed(ctx, UserID(userID), AgentID(agentID), ToolSetID(toolSetID))
}

// HasPermission reports whether a specific tool is not blocked. Falls
// back to the tool-set-level decision when no individual override exists.
func (s *Service) HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool {
	return s.store.IsToolAllowed(ctx, UserID(userID), AgentID(agentID), ToolSetID(toolSetID), ToolID(toolID))
}

// CheckConfirmation determines whether a tool call needs user confirmation
// for the given user/agent/tool. Used by the adapter layer to build
// ADK-native BeforeToolCallbacks that call ctx.RequestConfirmation().
func (s *Service) CheckConfirmation(ctx context.Context, userID, agentID, toolSetID, toolName string, args map[string]any) (bool, string) {
	return CheckConfirmation(ctx, s.store, s, UserID(userID), AgentID(agentID), ToolSetID(toolSetID), toolName, args)
}

// BlockMiddleware returns an [agent.Middleware] scoped to the given
// agent that prevents execution of tools the user has blocked.
// Defense-in-depth: blocked tools are normally filtered at resolution
// time, but this catches race conditions or resolution bugs.
func (s *Service) BlockMiddleware(agentID string) agent.Middleware {
	return BlockMiddleware(s, AgentID(agentID))
}

// IsToolInToolSet checks if a tool belongs to a specific toolset, looking
// the membership up through the registry.
//
// Lives on Service (not the store) because it consults the tool registry,
// which is an evaluation-time dependency distinct from the permission
// database.
func (s *Service) IsToolInToolSet(ctx context.Context, toolSetID ToolSetID, toolName string) (bool, error) {
	if s.toolRegistry == nil {
		return false, nil
	}
	tools, err := s.toolRegistry.GetToolsInSet(ctx, string(toolSetID))
	if err != nil {
		return false, err
	}
	for _, t := range tools {
		if t.ID == toolName || t.Name == toolName {
			return true, nil
		}
	}
	return false, nil
}
