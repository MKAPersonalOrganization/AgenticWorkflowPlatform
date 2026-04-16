package toolreqs

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/logger"
	"aicompass/foundation/permcache"
	"aicompass/foundation/toolregistry"
	"context"
	"fmt"
	"time"

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
// It also exposes the [CheckConfirmation] method for the adapter layer to
// build ADK-native confirmation callbacks.
type Service struct {
	log        *logger.Logger
	evaluator  *Evaluator
	permReq    *PermissionRequirement
	confirmReq *ConfirmationRequirement

	// Owned persistence — no delegation.
	repo         Repository
	toolSetCache *permcache.Cache[ToolSetPermission]
	toolCache    *permcache.Cache[Permission]

	// External dependency for tool membership lookup.
	toolRegistry *toolregistry.Registry
}

// ServiceConfig contains dependencies for creating a [Service].
type ServiceConfig struct {
	Log          *logger.Logger
	Repo         Repository
	RDB          redis.Cmdable         // Redis client for distributed caching
	ToolRegistry *toolregistry.Registry // For checking tool membership in toolsets
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
		toolRegistry: cfg.ToolRegistry,
	}

	// Wire the permission requirement to use this service's cached lookups.
	permReq.setChecker(s)

	return s
}

// =============================================================================
// Tool resolution — which tools should be available to the LLM
// =============================================================================

// GetEnabledToolSets returns the tool set IDs that are allowed (not blocked)
// for the given user/agent. This is the primary method used during tool resolution.
func (s *Service) GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	return s.getAllowedToolSets(ctx, userID, agentID)
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

// CheckConfirmation determines whether a tool call needs user confirmation
// for the given user/agent/tool combination. Checks both per-user DB config
// and admin-level global policies.
//
// Returns (needsConfirmation, hint). Used by the adapter layer to build
// ADK-native BeforeToolCallbacks that use ctx.RequestConfirmation().
func (s *Service) CheckConfirmation(ctx context.Context, userID, agentID, toolSetID, toolName string, args map[string]any) (bool, string) {
	s.log.Info(ctx, "CheckConfirmation called",
		"user_id", userID,
		"agent_id", agentID,
		"tool_set_id", toolSetID,
		"tool_name", toolName,
	)

	// Check all tool-sets with require_confirmation for this user/agent
	confirmSets, err := s.GetToolSetsWithConfirmation(ctx, userID, agentID)
	if err == nil {
		s.log.Info(ctx, "Tool sets with require_confirmation",
			"user_id", userID,
			"agent_id", agentID,
			"tool_sets", confirmSets,
		)
	}

	needsConfirm, hint := CheckConfirmation(ctx, s, s.confirmReq, userID, agentID, toolSetID, toolName, args)

	s.log.Info(ctx, "CheckConfirmation result",
		"user_id", userID,
		"agent_id", agentID,
		"tool_name", toolName,
		"needs_confirmation", needsConfirm,
		"hint", hint,
	)

	return needsConfirm, hint
}

// BlockMiddleware returns a [agent.Middleware] scoped to the given agent that
// prevents execution of tools the user has blocked. This is a defense-in-depth
// safety net: normally blocked tools are filtered at resolution time, but this
// catches race conditions or resolution bugs.
func (s *Service) BlockMiddleware(agentID string) agent.Middleware {
	return BlockMiddleware(s, agentID)
}

// RequiresConfirmation checks if a specific tool call requires user confirmation.
// This is a convenience method used by the adapter's before-tool callback.
func (s *Service) RequiresConfirmation(ctx context.Context, agentID, toolName string, args map[string]any) bool {
	return s.confirmReq.RequiresConfirmation(agentID, toolName, args)
}

// IsToolInToolSet checks if a tool belongs to a specific toolset.
// This is used during confirmation checks to avoid requiring confirmation
// for tools that are NOT members of a toolset with require_confirmation.
func (s *Service) IsToolInToolSet(ctx context.Context, userID, agentID, toolSetID, toolName string) (bool, error) {
	if s.toolRegistry == nil {
		// No registry available — cannot verify. Return false to avoid false positives.
		return false, nil
	}
	tools, err := s.toolRegistry.GetToolsInSet(ctx, toolSetID)
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
// Write operations — tool set permissions (REMOVED: use SetToolSetRequirementConfig)
// =============================================================================

// =============================================================================
// Requirement config — per-user, per-agent, per-tool requirements
// =============================================================================

// GetToolRequirementConfig retrieves the requirement configuration for a
// specific tool within a tool set for the given user/agent. Returns a zero
// config when no explicit configuration exists.
func (s *Service) GetToolRequirementConfig(ctx context.Context, userID, agentID, toolSetID, toolID string) (ToolRequirementConfig, error) {
	perm, err := s.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		// No permission record → default config
		return ToolRequirementConfig{}, nil
	}
	return perm.RequirementConfig(), nil
}

// SetToolRequirementConfig updates the requirement configuration for a
// specific tool within a tool set. The tool's enabled state is preserved;
// only the config column is updated.
func (s *Service) SetToolRequirementConfig(ctx context.Context, userID, agentID, toolSetID, toolID string, cfg ToolRequirementConfig) error {
	now := time.Now()

	// Load existing permission to preserve state.
	existing, err := s.repo.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		// No existing record → create one.
		existing = &Permission{
			UserID:    userID,
			AgentID:   agentID,
			ToolSetID: toolSetID,
			ToolID:    toolID,
			Enabled:   true,
			CreatedAt: now,
		}
	}

	existing.Config = cfg.MarshalConfig()
	existing.UpdatedAt = now

	if err := s.repo.SavePermission(ctx, existing); err != nil {
		return fmt.Errorf("failed to save tool requirement config: %w", err)
	}

	s.toolCache.Set(userID, existing, agentID, toolSetID, toolID)

	s.log.Info(ctx, "Tool requirement config updated",
		"user_id", userID, "agent_id", agentID,
		"tool_set_id", toolSetID, "tool_id", toolID,
		"action", cfg.Action, "require_confirmation", cfg.RequireConfirmation)
	return nil
}

// GetToolSetRequirementConfig retrieves the requirement configuration for an
// entire tool set for the given user/agent.
func (s *Service) GetToolSetRequirementConfig(ctx context.Context, userID, agentID, toolSetID string) (ToolRequirementConfig, error) {
	perm, err := s.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return ToolRequirementConfig{}, nil
	}
	return perm.RequirementConfig(), nil
}

// SetToolSetRequirementConfig updates the requirement configuration for an
// entire tool set.
func (s *Service) SetToolSetRequirementConfig(ctx context.Context, userID, agentID, toolSetID string, cfg ToolRequirementConfig) error {
	now := time.Now()

	existing, err := s.repo.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		existing = &ToolSetPermission{
			UserID:    userID,
			AgentID:   agentID,
			ToolSetID: toolSetID,
			Enabled:   true,
			CreatedAt: now,
		}
	}

	existing.Config = cfg.MarshalConfig()
	existing.UpdatedAt = now

	if err := s.repo.SaveToolSetPermission(ctx, existing); err != nil {
		return fmt.Errorf("failed to save tool set requirement config: %w", err)
	}

	s.toolSetCache.Set(userID, existing, agentID, toolSetID)

	s.log.Info(ctx, "Tool set requirement config updated",
		"user_id", userID, "agent_id", agentID, "tool_set_id", toolSetID,
		"action", cfg.Action, "require_confirmation", cfg.RequireConfirmation)
	return nil
}

// ListToolRequirementConfigs returns all tool requirement configs for a
// specific tool set of an agent. Returns a map of toolID → ToolRequirementConfig.
func (s *Service) ListToolRequirementConfigs(ctx context.Context, userID, agentID, toolSetID string) (map[string]ToolRequirementConfig, error) {
	perms, err := s.repo.ListPermissions(ctx, userID, agentID, toolSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool permissions: %w", err)
	}

	configs := make(map[string]ToolRequirementConfig, len(perms))
	for _, p := range perms {
		cfg := p.RequirementConfig()
		if !cfg.IsEmpty() {
			configs[p.ToolID] = cfg
		}
	}
	return configs, nil
}

// =============================================================================
// JSON-based requirement config — satisfies chatcontracts.ToolRequirementConfigService
// =============================================================================

// GetToolRequirementConfigJSON returns the requirement config JSON for a tool.
func (s *Service) GetToolRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID, toolID string) (string, error) {
	cfg, err := s.GetToolRequirementConfig(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		return "{}", err
	}
	return cfg.MarshalConfig(), nil
}

// SetToolRequirementConfigJSON saves the requirement config from raw JSON.
func (s *Service) SetToolRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID, toolID string, configJSON string) error {
	cfg := ParseToolRequirementConfig(configJSON)
	return s.SetToolRequirementConfig(ctx, userID, agentID, toolSetID, toolID, cfg)
}

// GetToolSetRequirementConfigJSON returns the requirement config JSON for a tool set.
func (s *Service) GetToolSetRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID string) (string, error) {
	cfg, err := s.GetToolSetRequirementConfig(ctx, userID, agentID, toolSetID)
	if err != nil {
		return "{}", err
	}
	return cfg.MarshalConfig(), nil
}

// SetToolSetRequirementConfigJSON saves the requirement config for a tool set from raw JSON.
func (s *Service) SetToolSetRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID string, configJSON string) error {
	cfg := ParseToolRequirementConfig(configJSON)
	return s.SetToolSetRequirementConfig(ctx, userID, agentID, toolSetID, cfg)
}

// ListToolRequirementConfigsJSON returns all tool requirement configs as toolID → JSON strings.
func (s *Service) ListToolRequirementConfigsJSON(ctx context.Context, userID, agentID, toolSetID string) (map[string]string, error) {
	configs, err := s.ListToolRequirementConfigs(ctx, userID, agentID, toolSetID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(configs))
	for k, v := range configs {
		result[k] = v.MarshalConfig()
	}
	return result, nil
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

// =============================================================================
// Internal permissionChecker implementation — used by PermissionRequirement
// These methods evaluate the config action (allow/block/require_confirmation)
// rather than the legacy Enabled boolean.
// =============================================================================

// isToolSetAllowed checks if a tool set is enabled.
// Implements [permissionChecker].
// Note: Tools are denied by default (fail-closed) unless explicitly allowed.
func (s *Service) isToolSetAllowed(ctx context.Context, userID, agentID, toolSetID string) bool {
	permission, err := s.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		// No permission record = denied (fail-closed)
		return false
	}
	cfg := permission.RequirementConfig()
	// Only "allow" and "require_confirmation" explicitly enable the tool set
	// "block" or empty/unspecified = denied
	return cfg.Action == RequirementActionAllow || cfg.Action == RequirementActionRequireConfirmation
}

// isToolAllowed checks if a specific tool is enabled.
// Falls back to tool-set level when no explicit individual config exists.
// Implements [permissionChecker].
// Note: Tools are denied by default (fail-closed) unless explicitly allowed.
func (s *Service) isToolAllowed(ctx context.Context, userID, agentID, toolSetID, toolID string) bool {
	permission, err := s.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		// No explicit tool config — inherit from tool set.
		return s.isToolSetAllowed(ctx, userID, agentID, toolSetID)
	}
	cfg := permission.RequirementConfig()
	// Only "allow" and "require_confirmation" explicitly enable the tool
	// "block" or empty/unspecified = denied
	return cfg.Action == RequirementActionAllow || cfg.Action == RequirementActionRequireConfirmation
}

// getAllowedToolSets returns all tool set IDs that are not explicitly blocked.
// Implements [permissionChecker].
// Note: Tools are denied by default (fail-closed). Only "allow" and "require_confirmation" enable a tool set.
func (s *Service) getAllowedToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	permissions, err := s.repo.ListToolSetPermissions(ctx, userID, agentID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load tool set permissions", "user_id", userID, "agent_id", agentID, "error", err)
		return nil, nil
	}

	// No permissions saved = all tools denied (fail-closed)
	if len(permissions) == 0 {
		return nil, nil
	}

	var allowedToolSets []string
	for _, perm := range permissions {
		cfg := perm.RequirementConfig()
		// Only "allow" and "require_confirmation" enable the tool set
		// "block" or empty/unspecified = denied
		if cfg.Action == RequirementActionAllow || cfg.Action == RequirementActionRequireConfirmation {
			allowedToolSets = append(allowedToolSets, perm.ToolSetID)
		}
	}

	return allowedToolSets, nil
}

// GetToolSetsWithConfirmation returns all tool set IDs that have require_confirmation
// action for the given user/agent. This is used to check if a tool needs confirmation
// when the tool-set ID is not known.
func (s *Service) GetToolSetsWithConfirmation(ctx context.Context, userID, agentID string) ([]string, error) {
	permissions, err := s.repo.ListToolSetPermissions(ctx, userID, agentID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load tool set permissions", "user_id", userID, "agent_id", agentID, "error", err)
		return nil, nil
	}

	if len(permissions) == 0 {
		s.log.Info(ctx, "No tool set permissions found", "user_id", userID, "agent_id", agentID)
		return nil, nil
	}

	var confirmToolSets []string
	for _, perm := range permissions {
		cfg := perm.RequirementConfig()
		if cfg.Action == RequirementActionRequireConfirmation || cfg.RequireConfirmation {
			confirmToolSets = append(confirmToolSets, perm.ToolSetID)
		}
	}

	s.log.Info(ctx, "Tool set permissions loaded",
		"user_id", userID,
		"agent_id", agentID,
		"total_permissions", len(permissions),
		"require_confirmation_sets", confirmToolSets,
	)

	return confirmToolSets, nil
}
