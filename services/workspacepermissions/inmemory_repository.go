package workspacepermissions

import (
	"context"
	"sync"
)

// InMemoryRepository is an in-memory implementation of the workspace permissions Repository.
type InMemoryRepository struct {
	mu   sync.RWMutex
	data map[string]map[string]*WorkspacePermission // agentID -> workspaceID -> Permission
}

// NewInMemoryRepository creates a new in-memory repository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		data: make(map[string]map[string]*WorkspacePermission),
	}
}

func (r *InMemoryRepository) GetPermission(_ context.Context, agentID, workspaceID string) (*WorkspacePermission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if agentPerms, ok := r.data[agentID]; ok {
		if perm, ok := agentPerms[workspaceID]; ok {
			return perm, nil
		}
	}
	return nil, nil
}

func (r *InMemoryRepository) ListPermissionsByAgent(_ context.Context, agentID string) ([]*WorkspacePermission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agentPerms, ok := r.data[agentID]
	if !ok {
		return nil, nil
	}

	perms := make([]*WorkspacePermission, 0, len(agentPerms))
	for _, perm := range agentPerms {
		perms = append(perms, perm)
	}
	return perms, nil
}

func (r *InMemoryRepository) SavePermission(_ context.Context, perm *WorkspacePermission) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.data[perm.AgentID] == nil {
		r.data[perm.AgentID] = make(map[string]*WorkspacePermission)
	}
	r.data[perm.AgentID][perm.WorkspaceID] = perm
	return nil
}

func (r *InMemoryRepository) DeletePermission(_ context.Context, agentID, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if agentPerms, ok := r.data[agentID]; ok {
		delete(agentPerms, workspaceID)
	}
	return nil
}
