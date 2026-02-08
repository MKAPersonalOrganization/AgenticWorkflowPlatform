package permissions

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryRepository implements Repository interface with in-memory storage.
type InMemoryRepository struct {
	mutex       sync.RWMutex
	permissions map[string]map[string]*Permission // agentID -> toolID -> Permission
}

// NewInMemoryRepository creates a new in-memory permissions repository.
func NewInMemoryRepository() Repository {
	return &InMemoryRepository{
		permissions: make(map[string]map[string]*Permission),
	}
}

// GetPermission retrieves a specific agent-tool permission.
func (r *InMemoryRepository) GetPermission(ctx context.Context, agentID, toolID string) (*Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agentPerms, exists := r.permissions[agentID]
	if !exists {
		return nil, fmt.Errorf("no permissions found for agent %s", agentID)
	}

	perm, exists := agentPerms[toolID]
	if !exists {
		return nil, fmt.Errorf("no permission found for agent %s and tool %s", agentID, toolID)
	}

	// Return a copy
	permCopy := *perm
	return &permCopy, nil
}

// ListPermissions returns all permissions for an agent.
func (r *InMemoryRepository) ListPermissions(ctx context.Context, agentID string) ([]Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	agentPerms, exists := r.permissions[agentID]
	if !exists {
		return []Permission{}, nil // Return empty slice, not an error
	}

	permissions := make([]Permission, 0, len(agentPerms))
	for _, perm := range agentPerms {
		permissions = append(permissions, *perm) // Copy to prevent external modifications
	}

	return permissions, nil
}

// SavePermission saves or updates a permission.
func (r *InMemoryRepository) SavePermission(ctx context.Context, permission *Permission) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Ensure agent entry exists
	if r.permissions[permission.AgentID] == nil {
		r.permissions[permission.AgentID] = make(map[string]*Permission)
	}

	// Create a copy to store
	permCopy := *permission
	r.permissions[permission.AgentID][permission.ToolID] = &permCopy

	return nil
}

// DeletePermission removes a specific agent-tool permission.
func (r *InMemoryRepository) DeletePermission(ctx context.Context, agentID, toolID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	agentPerms, exists := r.permissions[agentID]
	if !exists {
		return fmt.Errorf("no permissions found for agent %s", agentID)
	}

	if _, exists := agentPerms[toolID]; !exists {
		return fmt.Errorf("no permission found for agent %s and tool %s", agentID, toolID)
	}

	delete(agentPerms, toolID)

	// Clean up empty agent entries
	if len(agentPerms) == 0 {
		delete(r.permissions, agentID)
	}

	return nil
}

// ListAllPermissions returns all permissions in the system.
func (r *InMemoryRepository) ListAllPermissions(ctx context.Context) ([]Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var allPermissions []Permission
	for _, agentPerms := range r.permissions {
		for _, perm := range agentPerms {
			allPermissions = append(allPermissions, *perm) // Copy to prevent external modifications
		}
	}

	return allPermissions, nil
}
