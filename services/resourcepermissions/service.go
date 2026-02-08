package resourcepermissions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"school-with-ai/foundation/logger"
)

// ResourcePermission represents permission for an agent to access a specific resource.
type ResourcePermission struct {
	AgentID    string    `json:"agent_id"`
	ResourceID string    `json:"resource_id"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Repository defines the storage interface for resource permission data.
type Repository interface {
	GetPermission(ctx context.Context, agentID, resourceID string) (*ResourcePermission, error)
	ListPermissionsByAgent(ctx context.Context, agentID string) ([]ResourcePermission, error)
	SavePermission(ctx context.Context, permission *ResourcePermission) error
	DeletePermission(ctx context.Context, agentID, resourceID string) error
}

// Service manages resource permissions for agents.
type Service struct {
	log   *logger.Logger
	repo  Repository
	mutex sync.RWMutex
	cache map[string]map[string]*ResourcePermission // agentID -> resourceID -> *ResourcePermission
}

// NewService creates a new resource permissions service.
func NewService(log *logger.Logger, repo Repository) *Service {
	return &Service{
		log:   log,
		repo:  repo,
		cache: make(map[string]map[string]*ResourcePermission),
	}
}

// SetPermission enables or disables a specific resource for an agent.
func (s *Service) SetPermission(ctx context.Context, agentID, resourceID string, enabled bool) error {
	now := time.Now()

	permission := &ResourcePermission{
		AgentID:    agentID,
		ResourceID: resourceID,
		Enabled:    enabled,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := s.repo.SavePermission(ctx, permission); err != nil {
		return fmt.Errorf("failed to save resource permission: %w", err)
	}

	s.mutex.Lock()
	if _, exists := s.cache[agentID]; !exists {
		s.cache[agentID] = make(map[string]*ResourcePermission)
	}
	s.cache[agentID][resourceID] = permission
	s.mutex.Unlock()

	s.log.Info(ctx, "Resource permission updated", "agent_id", agentID, "resource_id", resourceID, "enabled", enabled)
	return nil
}

// HasPermission checks if an agent has permission to access a specific resource.
func (s *Service) HasPermission(ctx context.Context, agentID, resourceID string) bool {
	permission, err := s.GetPermission(ctx, agentID, resourceID)
	if err != nil {
		return false
	}
	return permission.Enabled
}

// GetPermission retrieves a specific agent's resource permission.
func (s *Service) GetPermission(ctx context.Context, agentID, resourceID string) (*ResourcePermission, error) {
	s.mutex.RLock()
	if agentPerms, exists := s.cache[agentID]; exists {
		if perm, exists := agentPerms[resourceID]; exists {
			s.mutex.RUnlock()
			permCopy := *perm
			return &permCopy, nil
		}
	}
	s.mutex.RUnlock()

	permission, err := s.repo.GetPermission(ctx, agentID, resourceID)
	if err != nil {
		return nil, err
	}

	s.mutex.Lock()
	if _, exists := s.cache[agentID]; !exists {
		s.cache[agentID] = make(map[string]*ResourcePermission)
	}
	s.cache[agentID][resourceID] = permission
	s.mutex.Unlock()

	return permission, nil
}

// ListPermissionsByAgent returns all resource permissions for an agent.
func (s *Service) ListPermissionsByAgent(ctx context.Context, agentID string) ([]ResourcePermission, error) {
	permissions, err := s.repo.ListPermissionsByAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list resource permissions: %w", err)
	}

	// Update cache with loaded permissions
	s.mutex.Lock()
	if _, exists := s.cache[agentID]; !exists {
		s.cache[agentID] = make(map[string]*ResourcePermission)
	}
	for i := range permissions {
		perm := permissions[i]
		s.cache[agentID][perm.ResourceID] = &perm
	}
	s.mutex.Unlock()

	return permissions, nil
}
