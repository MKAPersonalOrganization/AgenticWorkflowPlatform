package permissions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/foundation/logger"
)

// Permission represents an agent's access permission to a specific tool.
type Permission struct {
	AgentID   string    `json:"agent_id"`
	ToolID    string    `json:"tool_id"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config,omitempty"` // JSON string for tool-specific config
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Repository defines the storage interface for permission data.
type Repository interface {
	// GetPermission retrieves a specific agent-tool permission.
	GetPermission(ctx context.Context, agentID, toolID string) (*Permission, error)

	// ListPermissions returns all permissions for an agent.
	ListPermissions(ctx context.Context, agentID string) ([]Permission, error)

	// SavePermission saves or updates a permission.
	SavePermission(ctx context.Context, permission *Permission) error

	// DeletePermission removes a specific agent-tool permission.
	DeletePermission(ctx context.Context, agentID, toolID string) error

	// ListAllPermissions returns all permissions in the system.
	ListAllPermissions(ctx context.Context) ([]Permission, error)
}

// Service manages agent-tool permissions.
type Service struct {
	log   *logger.Logger
	repo  Repository
	mutex sync.RWMutex
	cache map[string]map[string]*Permission // agentID -> toolID -> Permission
}

// NewService creates a new permissions service.
func NewService(log *logger.Logger, repo Repository) *Service {
	return &Service{
		log:   log,
		repo:  repo,
		cache: make(map[string]map[string]*Permission),
	}
}

// Initialize loads existing permissions and sets up defaults.
func (s *Service) Initialize(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Load all existing permissions into cache
	permissions, err := s.repo.ListAllPermissions(ctx)
	if err != nil {
		return fmt.Errorf("failed to load permissions: %w", err)
	}

	for _, perm := range permissions {
		if s.cache[perm.AgentID] == nil {
			s.cache[perm.AgentID] = make(map[string]*Permission)
		}
		permCopy := perm
		s.cache[perm.AgentID][perm.ToolID] = &permCopy
	}

	s.log.Info(ctx, "Permissions service initialized", "permission_count", len(permissions))
	return nil
}

// SetPermission grants or revokes permission for an agent to use a tool.
func (s *Service) SetPermission(ctx context.Context, agentID, toolID string, enabled bool) error {
	now := time.Now()

	// Check if permission already exists
	existing, _ := s.GetPermission(ctx, agentID, toolID)

	permission := &Permission{
		AgentID:   agentID,
		ToolID:    toolID,
		Enabled:   enabled,
		UpdatedAt: now,
	}

	if existing != nil {
		permission.CreatedAt = existing.CreatedAt
		permission.Config = existing.Config
	} else {
		permission.CreatedAt = now
	}

	if err := s.repo.SavePermission(ctx, permission); err != nil {
		return fmt.Errorf("failed to save permission: %w", err)
	}

	// Update cache
	s.mutex.Lock()
	if s.cache[agentID] == nil {
		s.cache[agentID] = make(map[string]*Permission)
	}
	s.cache[agentID][toolID] = permission
	s.mutex.Unlock()

	s.log.Info(ctx, "Permission updated",
		"agent_id", agentID, "tool_id", toolID, "enabled", enabled)

	return nil
}

// GetPermission retrieves a specific agent-tool permission.
func (s *Service) GetPermission(ctx context.Context, agentID, toolID string) (*Permission, error) {
	s.mutex.RLock()
	if agentPerms, exists := s.cache[agentID]; exists {
		if perm, exists := agentPerms[toolID]; exists {
			s.mutex.RUnlock()
			permCopy := *perm
			return &permCopy, nil
		}
	}
	s.mutex.RUnlock()

	// Not in cache, try repository
	perm, err := s.repo.GetPermission(ctx, agentID, toolID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.mutex.Lock()
	if s.cache[agentID] == nil {
		s.cache[agentID] = make(map[string]*Permission)
	}
	s.cache[agentID][toolID] = perm
	s.mutex.Unlock()

	return perm, nil
}

// HasPermission checks if an agent has permission to use a specific tool.
func (s *Service) HasPermission(ctx context.Context, agentID, toolID string) bool {
	perm, err := s.GetPermission(ctx, agentID, toolID)
	if err != nil {
		// No explicit permission found, check default permissions
		return s.hasDefaultPermission(agentID, toolID)
	}
	return perm.Enabled
}

// ListAgentPermissions returns all permissions for a specific agent.
func (s *Service) ListAgentPermissions(ctx context.Context, agentID string) ([]Permission, error) {
	s.mutex.RLock()
	if agentPerms, exists := s.cache[agentID]; exists {
		permissions := make([]Permission, 0, len(agentPerms))
		for _, perm := range agentPerms {
			permissions = append(permissions, *perm)
		}
		s.mutex.RUnlock()
		return permissions, nil
	}
	s.mutex.RUnlock()

	// Load from repository
	permissions, err := s.repo.ListPermissions(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Cache the results
	s.mutex.Lock()
	if s.cache[agentID] == nil {
		s.cache[agentID] = make(map[string]*Permission)
	}
	for _, perm := range permissions {
		permCopy := perm
		s.cache[agentID][perm.ToolID] = &permCopy
	}
	s.mutex.Unlock()

	return permissions, nil
}

// GetEnabledTools returns a list of tool IDs that the agent has permission to use.
func (s *Service) GetEnabledTools(ctx context.Context, agentID string) ([]string, error) {
	// Start with default tools for this agent
	defaultTools := s.getDefaultToolsForAgent(agentID)
	enabledTools := make(map[string]bool)

	// Initialize with defaults
	for _, toolID := range defaultTools {
		enabledTools[toolID] = true
	}

	// Get explicit permissions and override defaults
	permissions, err := s.ListAgentPermissions(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Apply explicit permissions (both enabled and disabled)
	for _, perm := range permissions {
		enabledTools[perm.ToolID] = perm.Enabled
	}

	// Convert back to slice of enabled tool IDs
	var result []string
	for toolID, enabled := range enabledTools {
		if enabled {
			result = append(result, toolID)
		}
	}

	return result, nil
}

// InitializeDefaultPermissions sets up default permissions for an agent if none exist.
func (s *Service) InitializeDefaultPermissions(ctx context.Context, agentID string) error {
	// Check if agent already has any permissions
	existing, err := s.ListAgentPermissions(ctx, agentID)
	if err != nil {
		return err
	}

	// Agent already has permissions configured
	if len(existing) > 0 {
		return nil
	}

	// Set default permissions
	now := time.Now()
	defaultTools := s.getDefaultToolsForAgent(agentID)

	for _, toolID := range defaultTools {
		permission := &Permission{
			AgentID:   agentID,
			ToolID:    toolID,
			Enabled:   true,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.repo.SavePermission(ctx, permission); err != nil {
			s.log.Error(ctx, "Failed to save default permission",
				"agent_id", agentID, "tool_id", toolID, "error", err)
			continue
		}

		// Update cache
		s.mutex.Lock()
		if s.cache[agentID] == nil {
			s.cache[agentID] = make(map[string]*Permission)
		}
		s.cache[agentID][toolID] = permission
		s.mutex.Unlock()
	}

	s.log.Info(ctx, "Default permissions initialized",
		"agent_id", agentID, "tool_count", len(defaultTools))

	return nil
}

// hasDefaultPermission checks if an agent has default permission to a tool.
func (s *Service) hasDefaultPermission(agentID, toolID string) bool {
	defaultTools := s.getDefaultToolsForAgent(agentID)
	for _, defaultTool := range defaultTools {
		if defaultTool == toolID {
			return true
		}
	}
	return false
}

// getDefaultToolsForAgent returns default tool IDs based on agent type.
func (s *Service) getDefaultToolsForAgent(agentID string) []string {
	// Define default tool sets per agent type
	defaultSets := map[string][]string{
		"math-instructor": {
			// Calculator tools
			"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power", "calc-sqrt",
			// Visualization tools
			"viz-plot", "viz-animate",
			// Document tools
			"doc-create", "doc-update",
			// Memory tools
			"mem-remember", "mem-list",
		},
		"physics-instructor": {
			// Calculator tools
			"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power", "calc-sqrt",
			// Physics tools
			"physics-kinematics", "physics-dynamics",
			// Visualization tools
			"viz-plot",
			// Document tools
			"doc-create", "doc-update",
			// Memory tools
			"mem-remember", "mem-list",
		},
		"chemistry-instructor": {
			// Calculator tools
			"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power",
			// Chemistry tools
			"chem-balance", "chem-molarity",
			// Document tools
			"doc-create", "doc-update",
			// Memory tools
			"mem-remember", "mem-list",
		},
		"statistics-instructor": {
			// Calculator tools
			"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power", "calc-sqrt",
			// Statistics tools
			"stats-mean", "stats-median", "stats-mode", "stats-variance", "stats-stddev",
			"stats-correlation", "stats-regression",
			// Visualization tools
			"viz-plot", "viz-animate",
			// Document tools
			"doc-create", "doc-update",
			// Memory tools
			"mem-remember", "mem-list",
		},
		"diagramming-instructor": {
			// Diagramming tools
			"diagram-mermaid",
			// Document tools
			"doc-create", "doc-update",
			// Memory tools
			"mem-remember", "mem-list",
		},
		"general-assistant": {
			// Basic calculator
			"calc-add", "calc-subtract", "calc-multiply", "calc-divide",
			// Document tools
			"doc-create", "doc-update",
			// Memory tools
			"mem-remember", "mem-list",
		},
		"data-analyst": {
			// Calculator tools
			"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power",
			// Statistics tools
			"stats-mean", "stats-median", "stats-variance", "stats-correlation", "stats-regression",
			// Visualization tools
			"viz-plot", "viz-animate",
			// Document tools
			"doc-create", "doc-update",
		},
	}

	if tools, exists := defaultSets[agentID]; exists {
		return tools
	}

	// Default fallback for unknown agents
	return []string{"calc-add", "calc-subtract", "mem-remember"}
}
