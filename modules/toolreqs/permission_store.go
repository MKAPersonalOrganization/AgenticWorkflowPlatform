package toolreqs

import (
	"aicompass/foundation/logger"
	"aicompass/foundation/permcache"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// =============================================================================
// PermissionStore — the persistence + cache surface for tool requirements
// =============================================================================
//
// PermissionStore owns the storage side of tool requirements: the
// PostgreSQL [Repository] and its Redis-backed caches. Its methods are
// all about persisting and reading per-user, per-agent, per-tool (and
// per-tool-set) configuration.
//
// This type is a peer to [Service], not a private helper. The split
// follows the cohesion principle:
//
//   - PermissionStore keeps the 15 related CRUD/query methods together
//     (they all read and write the same rows and touch the same caches).
//   - Service keeps the unrelated surface far: the evaluator pipeline,
//     confirmation policies, and block middleware don't belong on a
//     type that also owns database persistence.
//
// Callers that only need to read or write requirement config (HTTP
// handlers in the chat module, for example) take a *PermissionStore
// directly. Callers that need to evaluate requirements at runtime (the
// adk adapter) take a *Service. Nothing needs both surfaces bundled.
type PermissionStore struct {
	log          *logger.Logger
	repo         Repository
	toolSetCache *permcache.Cache[ToolSetPermission]
	toolCache    *permcache.Cache[Permission]
}

// NewPermissionStore constructs a PermissionStore with the given repository
// and Redis client. The caches are namespaced so the store can share a
// Redis instance with other permcache users without key collisions.
func NewPermissionStore(log *logger.Logger, repo Repository, rdb redis.Cmdable) *PermissionStore {
	return &PermissionStore{
		log:          log,
		repo:         repo,
		toolSetCache: permcache.New[ToolSetPermission](rdb, "toolsetperm"),
		toolCache:    permcache.New[Permission](rdb, "toolperm"),
	}
}

// =============================================================================
// Read operations — tool set + tool permissions (with Redis caching)
// =============================================================================

// GetToolSetPermission retrieves a specific agent's tool set permission.
func (s *PermissionStore) GetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) (*ToolSetPermission, error) {
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
func (s *PermissionStore) GetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) (*Permission, error) {
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
// Requirement config — per-user, per-agent, per-tool / per-tool-set
// =============================================================================

// GetToolRequirementConfig retrieves the requirement configuration for a
// specific tool within a tool set for the given user/agent. Returns a zero
// config when no explicit configuration exists.
func (s *PermissionStore) GetToolRequirementConfig(ctx context.Context, userID, agentID, toolSetID, toolID string) (ToolRequirementConfig, error) {
	perm, err := s.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		return ToolRequirementConfig{}, nil
	}
	return perm.RequirementConfig(), nil
}

// SetToolRequirementConfig updates the requirement configuration for a
// specific tool within a tool set.
func (s *PermissionStore) SetToolRequirementConfig(ctx context.Context, userID, agentID, toolSetID, toolID string, cfg ToolRequirementConfig) error {
	now := time.Now()

	existing, err := s.repo.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		existing = &Permission{
			UserID:    userID,
			AgentID:   agentID,
			ToolSetID: toolSetID,
			ToolID:    toolID,
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
func (s *PermissionStore) GetToolSetRequirementConfig(ctx context.Context, userID, agentID, toolSetID string) (ToolRequirementConfig, error) {
	perm, err := s.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return ToolRequirementConfig{}, nil
	}
	return perm.RequirementConfig(), nil
}

// SetToolSetRequirementConfig updates the requirement configuration for an
// entire tool set.
func (s *PermissionStore) SetToolSetRequirementConfig(ctx context.Context, userID, agentID, toolSetID string, cfg ToolRequirementConfig) error {
	now := time.Now()

	existing, err := s.repo.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		existing = &ToolSetPermission{
			UserID:    userID,
			AgentID:   agentID,
			ToolSetID: toolSetID,
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
// specific tool set of an agent.
func (s *PermissionStore) ListToolRequirementConfigs(ctx context.Context, userID, agentID, toolSetID string) (map[string]ToolRequirementConfig, error) {
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
func (s *PermissionStore) GetToolRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID, toolID string) (string, error) {
	cfg, err := s.GetToolRequirementConfig(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		return "{}", err
	}
	return cfg.MarshalConfig(), nil
}

// SetToolRequirementConfigJSON saves the requirement config from raw JSON.
func (s *PermissionStore) SetToolRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID, toolID string, configJSON string) error {
	cfg := ParseToolRequirementConfig(configJSON)
	return s.SetToolRequirementConfig(ctx, userID, agentID, toolSetID, toolID, cfg)
}

// GetToolSetRequirementConfigJSON returns the requirement config JSON for a tool set.
func (s *PermissionStore) GetToolSetRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID string) (string, error) {
	cfg, err := s.GetToolSetRequirementConfig(ctx, userID, agentID, toolSetID)
	if err != nil {
		return "{}", err
	}
	return cfg.MarshalConfig(), nil
}

// SetToolSetRequirementConfigJSON saves the requirement config for a tool set from raw JSON.
func (s *PermissionStore) SetToolSetRequirementConfigJSON(ctx context.Context, userID, agentID, toolSetID string, configJSON string) error {
	cfg := ParseToolRequirementConfig(configJSON)
	return s.SetToolSetRequirementConfig(ctx, userID, agentID, toolSetID, cfg)
}

// ListToolRequirementConfigsJSON returns all tool requirement configs as
// toolID → JSON strings.
func (s *PermissionStore) ListToolRequirementConfigsJSON(ctx context.Context, userID, agentID, toolSetID string) (map[string]string, error) {
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
// Evaluation support — consumed by Service at runtime
// =============================================================================

// IsToolSetAllowed reports whether a tool set is enabled for this user/agent.
// Tools are denied by default (fail-closed) unless explicitly allowed.
func (s *PermissionStore) IsToolSetAllowed(ctx context.Context, userID, agentID, toolSetID string) bool {
	permission, err := s.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return false
	}
	cfg := permission.RequirementConfig()
	return cfg.Action == RequirementActionAllow || cfg.Action == RequirementActionRequireConfirmation
}

// IsToolAllowed reports whether a specific tool is enabled. Falls back to
// tool-set level when no explicit individual config exists.
func (s *PermissionStore) IsToolAllowed(ctx context.Context, userID, agentID, toolSetID, toolID string) bool {
	permission, err := s.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		return s.IsToolSetAllowed(ctx, userID, agentID, toolSetID)
	}
	cfg := permission.RequirementConfig()
	return cfg.Action == RequirementActionAllow || cfg.Action == RequirementActionRequireConfirmation
}

// AllowedToolSets returns all tool set IDs that are not explicitly blocked.
// Tools are denied by default (fail-closed).
func (s *PermissionStore) AllowedToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
	permissions, err := s.repo.ListToolSetPermissions(ctx, userID, agentID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load tool set permissions", "user_id", userID, "agent_id", agentID, "error", err)
		return nil, nil
	}

	if len(permissions) == 0 {
		return nil, nil
	}

	var allowed []string
	for _, perm := range permissions {
		cfg := perm.RequirementConfig()
		if cfg.Action == RequirementActionAllow || cfg.Action == RequirementActionRequireConfirmation {
			allowed = append(allowed, perm.ToolSetID)
		}
	}
	return allowed, nil
}

// ToolSetsWithConfirmation returns all tool set IDs that have
// require_confirmation action for the given user/agent. Used when the
// tool-set ID is unknown and the caller needs to scan candidates.
func (s *PermissionStore) ToolSetsWithConfirmation(ctx context.Context, userID, agentID string) ([]string, error) {
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

