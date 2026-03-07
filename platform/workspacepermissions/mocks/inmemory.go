package mocks

import (
	"context"
	"school-with-ai/platform/workspacepermissions"
	"sync"
)

// InMemoryRepository is an in-memory implementation of the workspace permissions Repository.
type InMemoryRepository struct {
	mu   sync.RWMutex
	data map[string]map[string]map[string]*workspacepermissions.WorkspacePermission // userID -> agentID -> workspaceID -> Permission
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		data: make(map[string]map[string]map[string]*workspacepermissions.WorkspacePermission),
	}
}

func (r *InMemoryRepository) GetPermission(_ context.Context, userID, agentID, workspaceID string) (*workspacepermissions.WorkspacePermission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if userPerms, ok := r.data[userID]; ok {
		if agentPerms, ok := userPerms[agentID]; ok {
			if perm, ok := agentPerms[workspaceID]; ok {
				return perm, nil
			}
		}
	}
	return nil, nil
}

func (r *InMemoryRepository) ListPermissionsByAgent(_ context.Context, userID, agentID string) ([]*workspacepermissions.WorkspacePermission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userPerms, ok := r.data[userID]
	if !ok {
		return nil, nil
	}

	agentPerms, ok := userPerms[agentID]
	if !ok {
		return nil, nil
	}

	perms := make([]*workspacepermissions.WorkspacePermission, 0, len(agentPerms))
	for _, perm := range agentPerms {
		perms = append(perms, perm)
	}
	return perms, nil
}

func (r *InMemoryRepository) SavePermission(_ context.Context, perm *workspacepermissions.WorkspacePermission) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.data[perm.UserID] == nil {
		r.data[perm.UserID] = make(map[string]map[string]*workspacepermissions.WorkspacePermission)
	}
	if r.data[perm.UserID][perm.AgentID] == nil {
		r.data[perm.UserID][perm.AgentID] = make(map[string]*workspacepermissions.WorkspacePermission)
	}
	r.data[perm.UserID][perm.AgentID][perm.WorkspaceID] = perm
	return nil
}

func (r *InMemoryRepository) DeletePermission(_ context.Context, userID, agentID, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if userPerms, ok := r.data[userID]; ok {
		if agentPerms, ok := userPerms[agentID]; ok {
			delete(agentPerms, workspaceID)
		}
	}
	return nil
}
