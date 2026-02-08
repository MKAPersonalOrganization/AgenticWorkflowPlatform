package resourcepermissions

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryRepository implements Repository interface with in-memory storage.
type InMemoryRepository struct {
	mutex       sync.RWMutex
	permissions map[string]map[string]*ResourcePermission // agentID -> resourceID -> ResourcePermission
}

// NewInMemoryRepository creates a new in-memory resource permissions repository.
func NewInMemoryRepository() Repository {
	return &InMemoryRepository{
		permissions: make(map[string]map[string]*ResourcePermission),
	}
}

// GetPermission retrieves a specific agent-resource permission.
func (r *InMemoryRepository) GetPermission(ctx context.Context, agentID, resourceID string) (*ResourcePermission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agentPerms, exists := r.permissions[agentID]
	if !exists {
		return nil, fmt.Errorf("no permissions found for agent %s", agentID)
	}

	perm, exists := agentPerms[resourceID]
	if !exists {
		return nil, fmt.Errorf("no permission found for agent %s, resource %s", agentID, resourceID)
	}

	permCopy := *perm
	return &permCopy, nil
}

// ListPermissionsByAgent returns all resource permissions for an agent.
func (r *InMemoryRepository) ListPermissionsByAgent(ctx context.Context, agentID string) ([]ResourcePermission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agentPerms, exists := r.permissions[agentID]
	if !exists {
		return []ResourcePermission{}, nil
	}

	permissions := make([]ResourcePermission, 0, len(agentPerms))
	for _, perm := range agentPerms {
		permissions = append(permissions, *perm)
	}

	return permissions, nil
}

// SavePermission saves or updates a resource permission.
func (r *InMemoryRepository) SavePermission(ctx context.Context, permission *ResourcePermission) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.permissions[permission.AgentID]; !exists {
		r.permissions[permission.AgentID] = make(map[string]*ResourcePermission)
	}

	permCopy := *permission
	r.permissions[permission.AgentID][permission.ResourceID] = &permCopy

	return nil
}

// DeletePermission removes a specific agent-resource permission.
func (r *InMemoryRepository) DeletePermission(ctx context.Context, agentID, resourceID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	agentPerms, exists := r.permissions[agentID]
	if !exists {
		return fmt.Errorf("no permissions found for agent %s", agentID)
	}

	if _, exists := agentPerms[resourceID]; !exists {
		return fmt.Errorf("no permission found for agent %s, resource %s", agentID, resourceID)
	}

	delete(agentPerms, resourceID)

	if len(agentPerms) == 0 {
		delete(r.permissions, agentID)
	}

	return nil
}
