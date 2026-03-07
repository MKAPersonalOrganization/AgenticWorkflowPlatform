package mocks

import (
	"context"
	"fmt"
	"school-with-ai/modules/permissions/tool"
	"sync"
)

// InMemoryRepository implements Repository interface with in-memory storage.
type InMemoryRepository struct {
	mutex              sync.RWMutex
	permissions        map[string]map[string]map[string]map[string]*tool.Permission // userID -> agentID -> toolSetID -> toolID -> Permission
	toolSetPermissions map[string]map[string]map[string]*tool.ToolSetPermission     // userID -> agentID -> toolSetID -> ToolSetPermission
}

// NewInMemoryRepository creates a new in-memory permissions repository.
func NewInMemoryRepository() tool.Repository {
	return &InMemoryRepository{
		permissions:        make(map[string]map[string]map[string]map[string]*tool.Permission),
		toolSetPermissions: make(map[string]map[string]map[string]*tool.ToolSetPermission),
	}
}

// =============================================================================
// Individual Tool Permission Methods
// =============================================================================

// GetPermission retrieves a specific agent-tool permission within a tool set.
func (r *InMemoryRepository) GetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) (*tool.Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.permissions[userID]
	if !exists {
		return nil, fmt.Errorf("no permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[agentID]
	if !exists {
		return nil, fmt.Errorf("no permissions found for agent %s", agentID)
	}

	toolSetPerms, exists := agentPerms[toolSetID]
	if !exists {
		return nil, fmt.Errorf("no permissions found for agent %s in tool set %s", agentID, toolSetID)
	}

	perm, exists := toolSetPerms[toolID]
	if !exists {
		return nil, fmt.Errorf("no permission found for agent %s, tool set %s, tool %s", agentID, toolSetID, toolID)
	}

	// Return a copy
	permCopy := *perm
	return &permCopy, nil
}

// ListPermissions returns all individual tool permissions for an agent within a tool set.
func (r *InMemoryRepository) ListPermissions(ctx context.Context, userID, agentID, toolSetID string) ([]tool.Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.permissions[userID]
	if !exists {
		return []tool.Permission{}, nil // Return empty slice, not an error
	}

	agentPerms, exists := userPerms[agentID]
	if !exists {
		return []tool.Permission{}, nil // Return empty slice, not an error
	}

	toolSetPerms, exists := agentPerms[toolSetID]
	if !exists {
		return []tool.Permission{}, nil // Return empty slice, not an error
	}

	permissionList := make([]tool.Permission, 0, len(toolSetPerms))
	for _, perm := range toolSetPerms {
		permissionList = append(permissionList, *perm) // Copy to prevent external modifications
	}

	return permissionList, nil
}

// SavePermission saves or updates an individual tool permission.
func (r *InMemoryRepository) SavePermission(ctx context.Context, permission *tool.Permission) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Ensure nested maps exist
	if r.permissions[permission.UserID] == nil {
		r.permissions[permission.UserID] = make(map[string]map[string]map[string]*tool.Permission)
	}
	if r.permissions[permission.UserID][permission.AgentID] == nil {
		r.permissions[permission.UserID][permission.AgentID] = make(map[string]map[string]*tool.Permission)
	}
	if r.permissions[permission.UserID][permission.AgentID][permission.ToolSetID] == nil {
		r.permissions[permission.UserID][permission.AgentID][permission.ToolSetID] = make(map[string]*tool.Permission)
	}

	// Create a copy to store
	permCopy := *permission
	r.permissions[permission.UserID][permission.AgentID][permission.ToolSetID][permission.ToolID] = &permCopy

	return nil
}

// DeletePermission removes a specific agent-tool permission within a tool set.
func (r *InMemoryRepository) DeletePermission(ctx context.Context, userID, agentID, toolSetID, toolID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	userPerms, exists := r.permissions[userID]
	if !exists {
		return fmt.Errorf("no permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[agentID]
	if !exists {
		return fmt.Errorf("no permissions found for agent %s", agentID)
	}

	toolSetPerms, exists := agentPerms[toolSetID]
	if !exists {
		return fmt.Errorf("no permissions found for agent %s in tool set %s", agentID, toolSetID)
	}

	if _, exists := toolSetPerms[toolID]; !exists {
		return fmt.Errorf("no permission found for agent %s, tool set %s, tool %s", agentID, toolSetID, toolID)
	}

	delete(toolSetPerms, toolID)

	// Clean up empty nested maps
	if len(toolSetPerms) == 0 {
		delete(agentPerms, toolSetID)
		if len(agentPerms) == 0 {
			delete(userPerms, agentID)
			if len(userPerms) == 0 {
				delete(r.permissions, userID)
			}
		}
	}

	return nil
}

// =============================================================================
// Tool Set Permission Methods
// =============================================================================

// GetToolSetPermission retrieves a specific agent-toolset permission.
func (r *InMemoryRepository) GetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) (*tool.ToolSetPermission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.toolSetPermissions[userID]
	if !exists {
		return nil, fmt.Errorf("no tool set permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[agentID]
	if !exists {
		return nil, fmt.Errorf("no tool set permissions found for agent %s", agentID)
	}

	perm, exists := agentPerms[toolSetID]
	if !exists {
		return nil, fmt.Errorf("no tool set permission found for agent %s and tool set %s", agentID, toolSetID)
	}

	// Return a copy
	permCopy := *perm
	return &permCopy, nil
}

// ListToolSetPermissions returns all tool set permissions for an agent.
func (r *InMemoryRepository) ListToolSetPermissions(ctx context.Context, userID, agentID string) ([]tool.ToolSetPermission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.toolSetPermissions[userID]
	if !exists {
		return []tool.ToolSetPermission{}, nil // Return empty slice, not an error
	}

	agentPerms, exists := userPerms[agentID]
	if !exists {
		return []tool.ToolSetPermission{}, nil // Return empty slice, not an error
	}

	permissionList := make([]tool.ToolSetPermission, 0, len(agentPerms))
	for _, perm := range agentPerms {
		permissionList = append(permissionList, *perm) // Copy to prevent external modifications
	}

	return permissionList, nil
}

// SaveToolSetPermission saves or updates a tool set permission.
func (r *InMemoryRepository) SaveToolSetPermission(ctx context.Context, permission *tool.ToolSetPermission) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.toolSetPermissions[permission.UserID] == nil {
		r.toolSetPermissions[permission.UserID] = make(map[string]map[string]*tool.ToolSetPermission)
	}

	if r.toolSetPermissions[permission.UserID][permission.AgentID] == nil {
		r.toolSetPermissions[permission.UserID][permission.AgentID] = make(map[string]*tool.ToolSetPermission)
	}

	// Create a copy to store
	permCopy := *permission
	r.toolSetPermissions[permission.UserID][permission.AgentID][permission.ToolSetID] = &permCopy

	return nil
}

// DeleteToolSetPermission removes a specific agent-toolset permission.
func (r *InMemoryRepository) DeleteToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	userPerms, exists := r.toolSetPermissions[userID]
	if !exists {
		return fmt.Errorf("no tool set permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[agentID]
	if !exists {
		return fmt.Errorf("no tool set permissions found for agent %s", agentID)
	}

	if _, exists := agentPerms[toolSetID]; !exists {
		return fmt.Errorf("no tool set permission found for agent %s and tool set %s", agentID, toolSetID)
	}

	delete(agentPerms, toolSetID)

	// Clean up empty agent map
	if len(agentPerms) == 0 {
		delete(userPerms, agentID)
		if len(userPerms) == 0 {
			delete(r.toolSetPermissions, userID)
		}
	}

	return nil
}
