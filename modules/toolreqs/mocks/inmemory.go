package mocks

import (
	"aicompass/modules/toolreqs"
	"context"
	"fmt"
	"sync"
)

// InMemoryRepository implements [toolreqs.Repository] with in-memory storage.
type InMemoryRepository struct {
	mutex              sync.RWMutex
	permissions        map[string]map[string]map[string]map[string]*toolreqs.Permission // userID -> agentID -> toolSetID -> toolID -> Permission
	toolSetPermissions map[string]map[string]map[string]*toolreqs.ToolSetPermission     // userID -> agentID -> toolSetID -> ToolSetPermission
}

// NewInMemoryRepository creates a new in-memory permissions repository.
func NewInMemoryRepository() toolreqs.Repository {
	return &InMemoryRepository{
		permissions:        make(map[string]map[string]map[string]map[string]*toolreqs.Permission),
		toolSetPermissions: make(map[string]map[string]map[string]*toolreqs.ToolSetPermission),
	}
}

// =============================================================================
// Individual Tool Permission Methods
// =============================================================================

// GetPermission retrieves a specific agent-tool permission within a tool set.
func (r *InMemoryRepository) GetPermission(_ context.Context, userID toolreqs.UserID, agentID toolreqs.AgentID, toolSetID toolreqs.ToolSetID, toolID toolreqs.ToolID) (*toolreqs.Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.permissions[string(userID)]
	if !exists {
		return nil, fmt.Errorf("no permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[string(agentID)]
	if !exists {
		return nil, fmt.Errorf("no permissions found for agent %s", agentID)
	}

	toolSetPerms, exists := agentPerms[string(toolSetID)]
	if !exists {
		return nil, fmt.Errorf("no permissions found for agent %s in tool set %s", agentID, toolSetID)
	}

	perm, exists := toolSetPerms[string(toolID)]
	if !exists {
		return nil, fmt.Errorf("no permission found for agent %s, tool set %s, tool %s", agentID, toolSetID, toolID)
	}

	permCopy := *perm
	return &permCopy, nil
}

// ListPermissions returns all individual tool permissions for an agent within a tool set.
func (r *InMemoryRepository) ListPermissions(_ context.Context, userID toolreqs.UserID, agentID toolreqs.AgentID, toolSetID toolreqs.ToolSetID) ([]toolreqs.Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.permissions[string(userID)]
	if !exists {
		return []toolreqs.Permission{}, nil
	}

	agentPerms, exists := userPerms[string(agentID)]
	if !exists {
		return []toolreqs.Permission{}, nil
	}

	toolSetPerms, exists := agentPerms[string(toolSetID)]
	if !exists {
		return []toolreqs.Permission{}, nil
	}

	permissionList := make([]toolreqs.Permission, 0, len(toolSetPerms))
	for _, perm := range toolSetPerms {
		permissionList = append(permissionList, *perm)
	}

	return permissionList, nil
}

// SavePermission saves or updates an individual tool permission.
func (r *InMemoryRepository) SavePermission(_ context.Context, permission *toolreqs.Permission) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.permissions[permission.UserID] == nil {
		r.permissions[permission.UserID] = make(map[string]map[string]map[string]*toolreqs.Permission)
	}
	if r.permissions[permission.UserID][permission.AgentID] == nil {
		r.permissions[permission.UserID][permission.AgentID] = make(map[string]map[string]*toolreqs.Permission)
	}
	if r.permissions[permission.UserID][permission.AgentID][permission.ToolSetID] == nil {
		r.permissions[permission.UserID][permission.AgentID][permission.ToolSetID] = make(map[string]*toolreqs.Permission)
	}

	permCopy := *permission
	r.permissions[permission.UserID][permission.AgentID][permission.ToolSetID][permission.ToolID] = &permCopy

	return nil
}

// DeletePermission removes a specific agent-tool permission within a tool set.
func (r *InMemoryRepository) DeletePermission(_ context.Context, userID toolreqs.UserID, agentID toolreqs.AgentID, toolSetID toolreqs.ToolSetID, toolID toolreqs.ToolID) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	userPerms, exists := r.permissions[string(userID)]
	if !exists {
		return fmt.Errorf("no permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[string(agentID)]
	if !exists {
		return fmt.Errorf("no permissions found for agent %s", agentID)
	}

	toolSetPerms, exists := agentPerms[string(toolSetID)]
	if !exists {
		return fmt.Errorf("no permissions found for agent %s in tool set %s", agentID, toolSetID)
	}

	if _, exists := toolSetPerms[string(toolID)]; !exists {
		return fmt.Errorf("no permission found for agent %s, tool set %s, tool %s", agentID, toolSetID, toolID)
	}

	delete(toolSetPerms, string(toolID))

	if len(toolSetPerms) == 0 {
		delete(agentPerms, string(toolSetID))
		if len(agentPerms) == 0 {
			delete(userPerms, string(agentID))
			if len(userPerms) == 0 {
				delete(r.permissions, string(userID))
			}
		}
	}

	return nil
}

// =============================================================================
// Tool Set Permission Methods
// =============================================================================

// GetToolSetPermission retrieves a specific agent-toolset permission.
func (r *InMemoryRepository) GetToolSetPermission(_ context.Context, userID toolreqs.UserID, agentID toolreqs.AgentID, toolSetID toolreqs.ToolSetID) (*toolreqs.ToolSetPermission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.toolSetPermissions[string(userID)]
	if !exists {
		return nil, fmt.Errorf("no tool set permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[string(agentID)]
	if !exists {
		return nil, fmt.Errorf("no tool set permissions found for agent %s", agentID)
	}

	perm, exists := agentPerms[string(toolSetID)]
	if !exists {
		return nil, fmt.Errorf("no tool set permission found for agent %s and tool set %s", agentID, toolSetID)
	}

	permCopy := *perm
	return &permCopy, nil
}

// ListToolSetPermissions returns all tool set permissions for an agent.
func (r *InMemoryRepository) ListToolSetPermissions(_ context.Context, userID toolreqs.UserID, agentID toolreqs.AgentID) ([]toolreqs.ToolSetPermission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	userPerms, exists := r.toolSetPermissions[string(userID)]
	if !exists {
		return []toolreqs.ToolSetPermission{}, nil
	}

	agentPerms, exists := userPerms[string(agentID)]
	if !exists {
		return []toolreqs.ToolSetPermission{}, nil
	}

	permissionList := make([]toolreqs.ToolSetPermission, 0, len(agentPerms))
	for _, perm := range agentPerms {
		permissionList = append(permissionList, *perm)
	}

	return permissionList, nil
}

// SaveToolSetPermission saves or updates a tool set permission.
func (r *InMemoryRepository) SaveToolSetPermission(_ context.Context, permission *toolreqs.ToolSetPermission) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.toolSetPermissions[permission.UserID] == nil {
		r.toolSetPermissions[permission.UserID] = make(map[string]map[string]*toolreqs.ToolSetPermission)
	}

	if r.toolSetPermissions[permission.UserID][permission.AgentID] == nil {
		r.toolSetPermissions[permission.UserID][permission.AgentID] = make(map[string]*toolreqs.ToolSetPermission)
	}

	permCopy := *permission
	r.toolSetPermissions[permission.UserID][permission.AgentID][permission.ToolSetID] = &permCopy

	return nil
}

// DeleteToolSetPermission removes a specific agent-toolset permission.
func (r *InMemoryRepository) DeleteToolSetPermission(_ context.Context, userID toolreqs.UserID, agentID toolreqs.AgentID, toolSetID toolreqs.ToolSetID) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	userPerms, exists := r.toolSetPermissions[string(userID)]
	if !exists {
		return fmt.Errorf("no tool set permissions found for user %s", userID)
	}

	agentPerms, exists := userPerms[string(agentID)]
	if !exists {
		return fmt.Errorf("no tool set permissions found for agent %s", agentID)
	}

	if _, exists := agentPerms[string(toolSetID)]; !exists {
		return fmt.Errorf("no tool set permission found for agent %s and tool set %s", agentID, toolSetID)
	}

	delete(agentPerms, string(toolSetID))

	if len(agentPerms) == 0 {
		delete(userPerms, string(agentID))
		if len(userPerms) == 0 {
			delete(r.toolSetPermissions, string(userID))
		}
	}

	return nil
}

// Verify interface compliance.
var _ toolreqs.Repository = (*InMemoryRepository)(nil)
