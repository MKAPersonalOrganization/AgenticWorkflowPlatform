package permissions

import (
	"context"
	"fmt"
	"school-with-ai/foundation/logger"
	"sync"
	"time"
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
	mutex        sync.RWMutex
	toolSetCache map[string]map[string]map[string]*ToolSetPermission     // userID -> agentID -> toolSetID -> ToolSetPermission
	toolCache    map[string]map[string]map[string]map[string]*Permission // userID -> agentID -> toolSetID -> toolID -> Permission
}

const permCacheMaxUsers = 5000 // Max distinct user entries in each cache map

// NewService creates a new permissions service.
func NewService(log *logger.Logger, repo Repository) *Service {
	return &Service{
		log:          log,
		repo:         repo,
		toolSetCache: make(map[string]map[string]map[string]*ToolSetPermission),
		toolCache:    make(map[string]map[string]map[string]map[string]*Permission),
	}
}

// evictCacheIfNeeded evicts user entries if the cache exceeds the max size.
// Must be called while holding s.mutex (write lock).
func (s *Service) evictToolSetCacheIfNeeded() {
	if len(s.toolSetCache) >= permCacheMaxUsers {
		// Evict half the entries (random, since map iteration is non-deterministic).
		count := 0
		half := len(s.toolSetCache) / 2
		for k := range s.toolSetCache {
			delete(s.toolSetCache, k)
			count++
			if count >= half {
				break
			}
		}
	}
}

func (s *Service) evictToolCacheIfNeeded() {
	if len(s.toolCache) >= permCacheMaxUsers {
		count := 0
		half := len(s.toolCache) / 2
		for k := range s.toolCache {
			delete(s.toolCache, k)
			count++
			if count >= half {
				break
			}
		}
	}
}

// Initialize loads existing tool set permissions.
func (s *Service) Initialize(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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

	// Update cache
	s.mutex.Lock()
	s.evictToolCacheIfNeeded()
	if _, exists := s.toolCache[userID]; !exists {
		s.toolCache[userID] = make(map[string]map[string]map[string]*Permission)
	}
	if _, exists := s.toolCache[userID][agentID]; !exists {
		s.toolCache[userID][agentID] = make(map[string]map[string]*Permission)
	}
	if _, exists := s.toolCache[userID][agentID][toolSetID]; !exists {
		s.toolCache[userID][agentID][toolSetID] = make(map[string]*Permission)
	}
	s.toolCache[userID][agentID][toolSetID][toolID] = permission
	s.mutex.Unlock()

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
	s.mutex.RLock()
	if userPerms, exists := s.toolCache[userID]; exists {
		if agentPerms, exists := userPerms[agentID]; exists {
			if toolSetPerms, exists := agentPerms[toolSetID]; exists {
				if perm, exists := toolSetPerms[toolID]; exists {
					s.mutex.RUnlock()
					permCopy := *perm
					return &permCopy, nil
				}
			}
		}
	}
	s.mutex.RUnlock()

	// Not in cache, load from repository
	permission, err := s.repo.GetPermission(ctx, userID, agentID, toolSetID, toolID)
	if err != nil {
		return nil, err
	}

	// Cache it
	s.mutex.Lock()
	s.evictToolCacheIfNeeded()
	if _, exists := s.toolCache[userID]; !exists {
		s.toolCache[userID] = make(map[string]map[string]map[string]*Permission)
	}
	if _, exists := s.toolCache[userID][agentID]; !exists {
		s.toolCache[userID][agentID] = make(map[string]map[string]*Permission)
	}
	if _, exists := s.toolCache[userID][agentID][toolSetID]; !exists {
		s.toolCache[userID][agentID][toolSetID] = make(map[string]*Permission)
	}
	s.toolCache[userID][agentID][toolSetID][toolID] = permission
	s.mutex.Unlock()

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

	// Update cache
	s.mutex.Lock()
	s.evictToolSetCacheIfNeeded()
	if _, exists := s.toolSetCache[userID]; !exists {
		s.toolSetCache[userID] = make(map[string]map[string]*ToolSetPermission)
	}
	if _, exists := s.toolSetCache[userID][agentID]; !exists {
		s.toolSetCache[userID][agentID] = make(map[string]*ToolSetPermission)
	}
	s.toolSetCache[userID][agentID][toolSetID] = permission
	s.mutex.Unlock()

	s.log.Info(ctx, "Tool set permission updated", "user_id", userID, "agent_id", agentID, "tool_set_id", toolSetID, "enabled", enabled)
	return nil
}

// GetToolSetPermission retrieves a specific agent's tool set permission.
func (s *Service) GetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) (*ToolSetPermission, error) {
	s.mutex.RLock()
	if userPerms, exists := s.toolSetCache[userID]; exists {
		if agentPerms, exists := userPerms[agentID]; exists {
			if perm, exists := agentPerms[toolSetID]; exists {
				s.mutex.RUnlock()
				permCopy := *perm
				return &permCopy, nil
			}
		}
	}
	s.mutex.RUnlock()

	// Not in cache, load from repository
	permission, err := s.repo.GetToolSetPermission(ctx, userID, agentID, toolSetID)
	if err != nil {
		return nil, err
	}

	// Cache it
	s.mutex.Lock()
	s.evictToolSetCacheIfNeeded()
	if _, exists := s.toolSetCache[userID]; !exists {
		s.toolSetCache[userID] = make(map[string]map[string]*ToolSetPermission)
	}
	if _, exists := s.toolSetCache[userID][agentID]; !exists {
		s.toolSetCache[userID][agentID] = make(map[string]*ToolSetPermission)
	}
	s.toolSetCache[userID][agentID][toolSetID] = permission
	s.mutex.Unlock()

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
