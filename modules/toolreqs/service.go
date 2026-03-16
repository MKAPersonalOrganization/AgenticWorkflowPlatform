package toolreqs

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/permcache"

	"github.com/redis/go-redis/v9"
)

// =============================================================================
// Service — the exclusive authority for tool requirements
// =============================================================================

// Service is the central API for all tool requirement decisions.
//
// Every tool access check — whether for resolution (which tools to show
// the LLM) or execution (whether to proceed with a tool call) — must go
// through this service.
//
// The service owns:
//   - [Repository]: persistence layer (PostgreSQL) for permission data
//   - Redis-backed caches for [Permission] and [ToolSetPermission] lookups
//   - [PermissionRequirement]: enable/disable evaluation
//   - [ConfirmationRequirement]: user approval policies (ADK-Go style)
//   - Any additional [ToolRequirement]s registered at startup or runtime
//
// It also exposes the [ConfirmationMiddleware] for the adapter layer to
// integrate into the tool execution pipeline.
type Service struct {
	log        *logger.Logger
	evaluator  *Evaluator
	permReq    *PermissionRequirement
	confirmReq *ConfirmationRequirement

	// Owned persistence — no delegation.
	repo         Repository
	toolSetCache *permcache.Cache[ToolSetPermission]
	toolCache    *permcache.Cache[Permission]
}

// ServiceConfig contains dependencies for creating a [Service].
type ServiceConfig struct {
	Log  *logger.Logger
	Repo Repository
	RDB  redis.Cmdable // Redis client for distributed caching
}

// NewService creates a new tool requirements service.
//
// The service is initialized with the built-in Permission and Confirmation
// requirements. Additional requirements can be added via [RegisterRequirement].
func NewService(cfg ServiceConfig) *Service {
	permReq := newPermissionRequirement()
	confirmReq := NewConfirmationRequirement()
	evaluator := NewEvaluator(permReq, confirmReq)

	s := &Service{
		log:          cfg.Log,
		evaluator:    evaluator,
		permReq:      permReq,
		confirmReq:   confirmReq,
		repo:         cfg.Repo,
		toolSetCache: permcache.New[ToolSetPermission](cfg.RDB, "toolsetperm"),
		toolCache:    permcache.New[Permission](cfg.RDB, "toolperm"),
	}

	// Wire the permission requirement to use this service's cached lookups.
	permReq.setChecker(s)

	return s
}

// =============================================================================
// Tool resolution — which tools should be available to the LLM
// =============================================================================

// GetEnabledToolSets returns the tool set IDs that are enabled for the
// given user/agent. This is the primary method used during tool resolution.
func (s *Service) GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	return s.getEnabledToolSets(ctx, userID, agentID)
}

// HasToolSetPermission checks if a tool set passes all requirements.
func (s *Service) HasToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) bool {
	result, err := s.evaluator.Evaluate(ctx, &ToolEvalRequest{
		UserID:    userID,
		AgentID:   agentID,
		ToolSetID: toolSetID,
	})
	if err != nil {
		return false
	}
	return result.Allowed
}

// HasPermission checks if a specific tool passes all requirements.
func (s *Service) HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool {
	result, err := s.evaluator.Evaluate(ctx, &ToolEvalRequest{
		UserID:    userID,
		AgentID:   agentID,
		ToolSetID: toolSetID,
		ToolID:    toolID,
		ToolName:  toolID,
	})
	if err != nil {
		return false
	}
	return result.Allowed
}

// EvaluateTool performs a full evaluation of all requirements for a tool call.
// This is used when you need the complete [ToolDecision] (including confirmation
// status, reasons, etc.).
func (s *Service) EvaluateTool(ctx context.Context, req *ToolEvalRequest) (*ToolDecision, error) {
	return s.evaluator.Evaluate(ctx, req)
}

// =============================================================================
// Confirmation management — ADK-Go style user approval
// =============================================================================

// Confirmation returns the [ConfirmationRequirement] for direct policy management.
//
// Usage:
//
//	svc.Confirmation().SetGlobalPolicy("file_delete", &toolreqs.ConfirmationPolicy{
//	    AlwaysConfirm: true,
//	    Hint:          "This tool will permanently delete files. Continue?",
//	})
func (s *Service) Confirmation() *ConfirmationRequirement {
	return s.confirmReq
}

// ConfirmationMiddleware returns a [agent.Middleware] scoped to the given agent
// that enforces user confirmation at tool execution time.
//
// The adapter should include this in the middleware chain for each agent run.
func (s *Service) ConfirmationMiddleware(agentID string) agent.Middleware {
	return ConfirmationMiddleware(s.confirmReq, agentID)
}

// RequiresConfirmation checks if a specific tool call requires user confirmation.
// This is a convenience method used by the adapter's before-tool callback.
func (s *Service) RequiresConfirmation(ctx context.Context, agentID, toolName string, args map[string]any) bool {
	return s.confirmReq.RequiresConfirmation(agentID, toolName, args)
}

// =============================================================================
// Extensibility — register new requirement types
// =============================================================================

// RegisterRequirement adds a new requirement to the evaluation pipeline.
//
// This is the extensibility point for future requirement types:
//   - Rate limiting
//   - Cost budgets
//   - Time-of-day restrictions
//   - Audit logging requirements
//   - etc.
func (s *Service) RegisterRequirement(req ToolRequirement) {
	s.evaluator.AddRequirement(req)
}

// =============================================================================
// Write operations — tool set permissions
// =============================================================================

// SetToolSetPermission enables or disables a tool set for an agent.
func (s *Service) SetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string, enabled bool) error {
	now := time.Now()

	permission := &ToolSetPermission{
		UserID:    userID,
		AgentID:   agentID,
		ToolSetID: toolSetID,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.SaveToolSetPermission(ctx, permission); err != nil {
		return fmt.Errorf("failed to save tool set permission: %w", err)
	}

	s.toolSetCache.Set(userID, permission, agentID, toolSetID)

	s.log.Info(ctx, "Tool set permission updated", "user_id", userID, "agent_id", agentID, "tool_set_id", toolSetID, "enabled", enabled)
	return nil
}

// SetPermission enables or disables an individual tool within a tool set.
func (s *Service) SetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string, enabled bool) error {
	now := time.Now()

	permission := &Permission{
		UserID:    userID,
		AgentID:   agentID,
		ToolSetID: toolSetID,
		ToolID:    toolID,
		Enabled:   enabled,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.SavePermission(ctx, permission); err != nil {
		return fmt.Errorf("failed to save individual tool permission: %w", err)
	}

	s.toolCache.Set(userID, permission, agentID, toolSetID, toolID)

	s.log.Info(ctx, "Individual tool permission updated", "user_id", userID, "agent_id", agentID, "tool_set_id", toolSetID, "tool_id", toolID, "enabled", enabled)
	return nil
}

// =============================================================================
// Read operations — tool set permissions (with Redis caching)
// =============================================================================

// GetToolSetPermission retrieves a specific agent's tool set permission.
func (s *Service) GetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) (*ToolSetPermission, error) {
	if cached, ok := s.toolSetCache.Get(userID, agentID, toolSetID); ok {
		permCopy := *cached
		return &permCopy, nil
	}

	permission, err := s.repo.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return nil, err
	}

	s.toolSetCache.Set(userID, permission, agentID, toolSetID)
	return permission, nil
}

// GetPermission retrieves a specific agent's tool permission within a tool set.
func (s *Service) GetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) (*Permission, error) {
	if cached, ok := s.toolCache.Get(userID, agentID, toolSetID, toolID); ok {
		permCopy := *cached
		return &permCopy, nil
	}

	permission, err := s.repo.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		return nil, err
	}

	s.toolCache.Set(userID, permission, agentID, toolSetID, toolID)
	return permission, nil
}

// GetEnabledToolsInSet returns all enabled tools within a specific tool set for an agent.
func (s *Service) GetEnabledToolsInSet(ctx context.Context, userID, agentID, toolSetID string) ([]string, error) {
	if !s.hasToolSetPermission(ctx, userID, agentID, toolSetID) {
		return []string{}, nil
	}

	permissions, err := s.repo.ListPermissions(ctx, userID, agentID, toolSetID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load individual tool permissions", "user_id", userID, "agent_id", agentID, "tool_set_id", toolSetID, "error", err)
		return []string{}, nil
	}

	var enabledTools []string
	for _, perm := range permissions {
		if perm.Enabled {
			enabledTools = append(enabledTools, perm.ToolID)
		}
	}

	return enabledTools, nil
}

// =============================================================================
// Internal permissionChecker implementation — used by PermissionRequirement
// =============================================================================

// hasToolSetPermission checks if an agent has permission to use a tool set.
// Implements [permissionChecker].
func (s *Service) hasToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) bool {
	permission, err := s.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return false
	}
	return permission.Enabled
}

// hasPermission checks if an agent has permission to use a specific tool.
// Falls back to tool-set level when no explicit individual override exists.
// Implements [permissionChecker].
func (s *Service) hasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool {
	permission, err := s.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		// No explicit permission — inherit from tool set.
		return s.hasToolSetPermission(ctx, userID, agentID, toolSetID)
	}
	return permission.Enabled
}

// getEnabledToolSets returns all enabled tool sets for an agent.
// Implements [permissionChecker].
func (s *Service) getEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	permissions, err := s.repo.ListToolSetPermissions(ctx, userID, agentID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load tool set permissions", "user_id", userID, "agent_id", agentID, "error", err)
		return nil, nil
	}

	var enabledToolSets []string
	for _, perm := range permissions {
		if perm.Enabled {
			enabledToolSets = append(enabledToolSets, perm.ToolSetID)
		}
	}

	return enabledToolSets, nil
}
