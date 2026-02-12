package edgequakepermissions

import (
	"fmt"
	"sync"

	"school-with-ai/domain"

	"github.com/google/uuid"
)

// InMemoryRepository implements domain.EdgequakePermissionRepository using in-memory maps.
type InMemoryRepository struct {
	mutex       sync.RWMutex
	tenants     map[uuid.UUID]*domain.EdgequakeTenant
	workspaces  map[uuid.UUID]*domain.EdgequakeWorkspace
	memberships map[string][]*domain.EdgequakeMembership // userID -> list of memberships
}

// NewInMemoryRepository creates a new instance of InMemoryRepository.
func NewInMemoryRepository() domain.EdgequakePermissionRepository {
	return &InMemoryRepository{
		tenants:     make(map[uuid.UUID]*domain.EdgequakeTenant),
		workspaces:  make(map[uuid.UUID]*domain.EdgequakeWorkspace),
		memberships: make(map[string][]*domain.EdgequakeMembership),
	}
}

// SaveTenant saves or updates a tenant.
func (r *InMemoryRepository) SaveTenant(tenant *domain.EdgequakeTenant) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.tenants[tenant.ID] = tenant
	return nil
}

// GetTenant retrieves a tenant by ID.
func (r *InMemoryRepository) GetTenant(id uuid.UUID) (*domain.EdgequakeTenant, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	tenant, ok := r.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant not found: %s", id)
	}
	return tenant, nil
}

// SaveWorkspace saves or updates a workspace.
func (r *InMemoryRepository) SaveWorkspace(workspace *domain.EdgequakeWorkspace) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.workspaces[workspace.ID] = workspace
	return nil
}

// GetWorkspace retrieves a workspace by ID.
func (r *InMemoryRepository) GetWorkspace(id uuid.UUID) (*domain.EdgequakeWorkspace, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	workspace, ok := r.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", id)
	}
	return workspace, nil
}

// SaveMembership saves or updates a membership.
func (r *InMemoryRepository) SaveMembership(membership *domain.EdgequakeMembership) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	list := r.memberships[membership.UserID]
	found := false
	for i, m := range list {
		if m.ID == membership.ID {
			list[i] = membership
			found = true
			break
		}
	}
	if !found {
		r.memberships[membership.UserID] = append(list, membership)
	}
	return nil
}

// ListMembershipsByUser retrieves all memberships for a user.
func (r *InMemoryRepository) ListMembershipsByUser(userID string) ([]*domain.EdgequakeMembership, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	list := r.memberships[userID]
	return list, nil
}
