package permissions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/foundation/logger"
)

// Permission represents permission for an agent to use a specific tool within a tool set.
type Permission struct {
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
	GetPermission(ctx context.Context, agentID, toolSetID, toolID string) (*Permission, error)

	// ListPermissions returns all individual tool permissions for an agent within a tool set.
	ListPermissions(ctx context.Context, agentID, toolSetID string) ([]Permission, error)

	// SavePermission saves or updates an individual tool permission.
	SavePermission(ctx context.Context, permission *Permission) error

	// DeletePermission removes a specific agent-tool permission.
	DeletePermission(ctx context.Context, agentID, toolSetID, toolID string) error

	// Tool Set Permissions
	// GetToolSetPermission retrieves a specific agent-toolset permission.
	GetToolSetPermission(ctx context.Context, agentID, toolSetID string) (*ToolSetPermission, error)

	// ListToolSetPermissions returns all tool set permissions for an agent.
	ListToolSetPermissions(ctx context.Context, agentID string) ([]ToolSetPermission, error)

	// SaveToolSetPermission saves or updates a tool set permission.
	SaveToolSetPermission(ctx context.Context, permission *ToolSetPermission) error

	// DeleteToolSetPermission removes a specific agent-toolset permission.
	DeleteToolSetPermission(ctx context.Context, agentID, toolSetID string) error
}

// Service manages tool set permissions and individual tool permissions within tool sets for agents.
type Service struct {
	log          *logger.Logger
	repo         Repository
	mutex        sync.RWMutex
	toolSetCache map[string]map[string]*ToolSetPermission     // agentID -> toolSetID -> ToolSetPermission
	toolCache    map[string]map[string]map[string]*Permission // agentID -> toolSetID -> toolID -> Permission
}

// NewService creates a new permissions service.
func NewService(log *logger.Logger, repo Repository) *Service {
	return &Service{
		log:          log,
		repo:         repo,
		toolSetCache: make(map[string]map[string]*ToolSetPermission),
		toolCache:    make(map[string]map[string]map[string]*Permission),
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
func (s *Service) SetPermission(ctx context.Context, agentID, toolSetID, toolID string, enabled bool) error {
	now := time.Now()

	permission := &Permission{
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
	if _, exists := s.toolCache[agentID]; !exists {
		s.toolCache[agentID] = make(map[string]map[string]*Permission)
	}
	if _, exists := s.toolCache[agentID][toolSetID]; !exists {
		s.toolCache[agentID][toolSetID] = make(map[string]*Permission)
	}
	s.toolCache[agentID][toolSetID][toolID] = permission
	s.mutex.Unlock()

	s.log.Info(ctx, "Individual tool permission updated", "agent_id", agentID, "tool_set_id", toolSetID, "tool_id", toolID, "enabled", enabled)
	return nil
}

// HasPermission checks if an agent has permission to use a specific tool within a tool set.
func (s *Service) HasPermission(ctx context.Context, agentID, toolSetID, toolID string) bool {
	permission, err := s.GetPermission(ctx, agentID, toolSetID, toolID)
	if err != nil {
		// If no explicit permission exists, check if the tool set is enabled (inherit from tool set)
		return s.HasToolSetPermission(ctx, agentID, toolSetID)
	}
	return permission.Enabled
}

// GetPermission retrieves a specific agent's tool permission within a tool set.
func (s *Service) GetPermission(ctx context.Context, agentID, toolSetID, toolID string) (*Permission, error) {
	s.mutex.RLock()
	if agentPerms, exists := s.toolCache[agentID]; exists {
		if toolSetPerms, exists := agentPerms[toolSetID]; exists {
			if perm, exists := toolSetPerms[toolID]; exists {
				s.mutex.RUnlock()
				permCopy := *perm
				return &permCopy, nil
			}
		}
	}
	s.mutex.RUnlock()

	// Not in cache, load from repository
	permission, err := s.repo.GetPermission(ctx, agentID, toolSetID, toolID)
	if err != nil {
		return nil, err
	}

	// Cache it
	s.mutex.Lock()
	if _, exists := s.toolCache[agentID]; !exists {
		s.toolCache[agentID] = make(map[string]map[string]*Permission)
	}
	if _, exists := s.toolCache[agentID][toolSetID]; !exists {
		s.toolCache[agentID][toolSetID] = make(map[string]*Permission)
	}
	s.toolCache[agentID][toolSetID][toolID] = permission
	s.mutex.Unlock()

	return permission, nil
}

// GetEnabledToolsInSet returns all enabled tools within a specific tool set for an agent.
func (s *Service) GetEnabledToolsInSet(ctx context.Context, agentID, toolSetID string) ([]string, error) {
	// First check if the tool set is enabled
	if !s.HasToolSetPermission(ctx, agentID, toolSetID) {
		return []string{}, nil
	}

	// Load individual tool permissions for this tool set
	permissions, err := s.repo.ListPermissions(ctx, agentID, toolSetID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load individual tool permissions", "agent_id", agentID, "tool_set_id", toolSetID, "error", err)
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
func (s *Service) SetToolSetPermission(ctx context.Context, agentID, toolSetID string, enabled bool) error {
	now := time.Now()

	permission := &ToolSetPermission{
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
	if _, exists := s.toolSetCache[agentID]; !exists {
		s.toolSetCache[agentID] = make(map[string]*ToolSetPermission)
	}
	s.toolSetCache[agentID][toolSetID] = permission
	s.mutex.Unlock()

	s.log.Info(ctx, "Tool set permission updated", "agent_id", agentID, "tool_set_id", toolSetID, "enabled", enabled)
	return nil
}

// GetToolSetPermission retrieves a specific agent's tool set permission.
func (s *Service) GetToolSetPermission(ctx context.Context, agentID, toolSetID string) (*ToolSetPermission, error) {
	s.mutex.RLock()
	if agentPerms, exists := s.toolSetCache[agentID]; exists {
		if perm, exists := agentPerms[toolSetID]; exists {
			s.mutex.RUnlock()
			permCopy := *perm
			return &permCopy, nil
		}
	}
	s.mutex.RUnlock()

	// Not in cache, load from repository
	permission, err := s.repo.GetToolSetPermission(ctx, agentID, toolSetID)
	if err != nil {
		return nil, err
	}

	// Cache it
	s.mutex.Lock()
	if _, exists := s.toolSetCache[agentID]; !exists {
		s.toolSetCache[agentID] = make(map[string]*ToolSetPermission)
	}
	s.toolSetCache[agentID][toolSetID] = permission
	s.mutex.Unlock()

	return permission, nil
}

// HasToolSetPermission checks if an agent has permission to use a tool set.
func (s *Service) HasToolSetPermission(ctx context.Context, agentID, toolSetID string) bool {
	permission, err := s.GetToolSetPermission(ctx, agentID, toolSetID)
	if err != nil {
		// If no explicit permission exists, check if this is a default tool set for the agent
		return s.hasDefaultToolSetPermission(agentID, toolSetID)
	}
	return permission.Enabled
}

// GetEnabledToolSets returns all enabled tool sets for an agent.
func (s *Service) GetEnabledToolSets(ctx context.Context, agentID string) ([]string, error) {
	// Get default tool sets for this agent type
	defaultToolSets := s.getDefaultToolSetsForAgent(agentID)

	// Create a map to track final permissions (default + explicit overrides)
	finalPermissions := make(map[string]bool)

	// Start with defaults
	for _, toolSetID := range defaultToolSets {
		finalPermissions[toolSetID] = true
	}

	// Load explicit permissions from database to override defaults
	permissions, err := s.repo.ListToolSetPermissions(ctx, agentID)
	if err != nil {
		s.log.Warn(ctx, "Failed to load tool set permissions", "agent_id", agentID, "error", err)
		// Return just defaults if database fails
		return defaultToolSets, nil
	}

	// Apply explicit permissions as overrides
	for _, perm := range permissions {
		finalPermissions[perm.ToolSetID] = perm.Enabled
	}

	// Build final list of enabled tools
	var enabledToolSets []string
	for toolSetID, enabled := range finalPermissions {
		if enabled {
			enabledToolSets = append(enabledToolSets, toolSetID)
		}
	}

	return enabledToolSets, nil
}

// hasDefaultToolSetPermission checks if a tool set is enabled by default for an agent.
func (s *Service) hasDefaultToolSetPermission(agentID, toolSetID string) bool {
	defaultToolSets := s.getDefaultToolSetsForAgent(agentID)
	for _, defaultToolSet := range defaultToolSets {
		if defaultToolSet == toolSetID {
			return true
		}
	}
	return false
}

// getDefaultToolSetsForAgent returns the default tool sets for each agent type.
func (s *Service) getDefaultToolSetsForAgent(agentID string) []string {
	defaultSets := map[string][]string{
		"math-instructor": {
			"calculator-set",
			"visualization-set",
			"memory-set",
		},
		"physics-instructor": {
			"calculator-set",
			"physics-set",
			"visualization-set",
			"memory-set",
		},
		"chemistry-instructor": {
			"calculator-set",
			"chemistry-set",
			"memory-set",
		},
		"statistics-instructor": {
			"calculator-set",
			"statistics-set",
			"visualization-set",
			"memory-set",
		},
		"diagramming-instructor": {
			"diagramming-set",
			"memory-set",
		},
	}

	if toolSets, exists := defaultSets[agentID]; exists {
		return toolSets
	}

	// Default fallback for unknown agents
	return []string{"calculator-set", "memory-set"}
}
