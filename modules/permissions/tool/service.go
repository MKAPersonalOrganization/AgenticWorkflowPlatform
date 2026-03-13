package tool

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/permcache"

	"github.com/redis/go-redis/v9"
)

// Permission represents permission for an agent to use a specific tool within a tool set.
type Permission struct {
	UserID    string    `json:"user_id"`
	AgentID   string    `json:"agent_id"`
	ToolSetID string    `json:"tool_set_id"`
	ToolID    string    `json:"tool_id"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToolSetPermission represents an agent's access permission to a tool set.
type ToolSetPermission struct {
	UserID    string    `json:"user_id"`
	AgentID   string    `json:"agent_id"`
	ToolSetID string    `json:"tool_set_id"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config,omitempty"` // JSON string for tool set-specific config
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Repository defines the storage interface for tool set and individual tool permission data.
type Repository interface {
	// Individual Tool Permissions within Tool Sets
	// GetPermission retrieves a specific agent-tool permission.
	GetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) (*Permission, error)

	// ListPermissions returns all individual tool permissions for an agent within a tool set.
	ListPermissions(ctx context.Context, userID, agentID, toolSetID string) ([]Permission, error)

	// SavePermission saves or updates an individual tool permission.
	SavePermission(ctx context.Context, permission *Permission) error

	// DeletePermission removes a specific agent-tool permission.
	DeletePermission(ctx context.Context, userID, agentID, toolSetID, toolID string) error

	// Tool Set Permissions
	// GetToolSetPermission retrieves a specific agent-toolset permission.
	GetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) (*ToolSetPermission, error)

	// ListToolSetPermissions returns all tool set permissions for an agent.
	ListToolSetPermissions(ctx context.Context, userID, agentID string) ([]ToolSetPermission, error)

	// SaveToolSetPermission saves or updates a tool set permission.
	SaveToolSetPermission(ctx context.Context, permission *ToolSetPermission) error

	// DeleteToolSetPermission removes a specific agent-toolset permission.
	DeleteToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) error
}

// Service manages tool set permissions and individual tool permissions within tool sets for agents.
type Service struct {
	log          *logger.Logger
	repo         Repository
	toolSetCache *permcache.Cache[ToolSetPermission]
	toolCache    *permcache.Cache[Permission]
}

// NewService creates a new permissions service.
// The rdb client is used for distributed Redis caching of permission lookups.
func NewService(log *logger.Logger, repo Repository, rdb redis.Cmdable) *Service {
	return &Service{
		log:          log,
		repo:         repo,
		toolSetCache: permcache.New[ToolSetPermission](rdb, "toolsetperm"),
		toolCache:    permcache.New[Permission](rdb, "toolperm"),
	}
}

// Initialize loads existing tool set permissions.
func (s *Service) Initialize(ctx context.Context) error {
	s.log.Info(ctx, "Permissions service initialized for tool sets")
	return nil
}

// =============================================================================
// Individual Tool Permission Management (within Tool Sets)
// =============================================================================

// SetPermission enables or disables a specific tool within a tool set for an agent.
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

// HasPermission checks if an agent has permission to use a specific tool within a tool set.
func (s *Service) HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool {
	permission, err := s.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		// If no explicit permission exists, check if the tool set is enabled (inherit from tool set)
		return s.HasToolSetPermission(ctx, userID, agentID, toolSetID)
	}
	return permission.Enabled
}

// GetPermission retrieves a specific agent's tool permission within a tool set.
func (s *Service) GetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) (*Permission, error) {
	if cached, ok := s.toolCache.Get(userID, agentID, toolSetID, toolID); ok {
		permCopy := *cached
		return &permCopy, nil
	}

	// Not in cache, load from repository
	permission, err := s.repo.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		return nil, err
	}

	s.toolCache.Set(userID, permission, agentID, toolSetID, toolID)
	return permission, nil
}

// GetEnabledToolsInSet returns all enabled tools within a specific tool set for an agent.
func (s *Service) GetEnabledToolsInSet(ctx context.Context, userID, agentID, toolSetID string) ([]string, error) {
	// First check if the tool set is enabled
	if !s.HasToolSetPermission(ctx, userID, agentID, toolSetID) {
		return []string{}, nil
	}

	// Load individual tool permissions for this tool set
	permissions, err := s.repo.ListPermissions(ctx, userID, agentID, toolSetID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load individual tool permissions", "user_id", userID, "agent_id", agentID, "tool_set_id", toolSetID, "error", err)
		// If we can't load individual permissions, return empty (tool set enabled but no individual overrides)
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
// Tool Set Permission Management
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

// GetToolSetPermission retrieves a specific agent's tool set permission.
func (s *Service) GetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) (*ToolSetPermission, error) {
	if cached, ok := s.toolSetCache.Get(userID, agentID, toolSetID); ok {
		permCopy := *cached
		return &permCopy, nil
	}

	// Not in cache, load from repository
	permission, err := s.repo.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return nil, err
	}

	s.toolSetCache.Set(userID, permission, agentID, toolSetID)
	return permission, nil
}

// HasToolSetPermission checks if an agent has permission to use a tool set.
func (s *Service) HasToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) bool {
	permission, err := s.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return false
	}
	return permission.Enabled
}

// GetEnabledToolSets returns all enabled tool sets for an agent.
// Permissions are purely explicit — there are no hardcoded defaults.
func (s *Service) GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error) {
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
