package workspacepermissions

import (
	"context"
	"sync"
	"time"

	"school-with-ai/foundation/logger"
)

// WorkspacePermission represents a workspace attached to an agent.
type WorkspacePermission struct {
	AgentID     string    `json:"agent_id"`
	WorkspaceID string    `json:"workspace_id"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repository defines the storage interface for workspace permissions.
type Repository interface {
	GetPermission(ctx context.Context, agentID, workspaceID string) (*WorkspacePermission, error)
	ListPermissionsByAgent(ctx context.Context, agentID string) ([]*WorkspacePermission, error)
	SavePermission(ctx context.Context, perm *WorkspacePermission) error
	DeletePermission(ctx context.Context, agentID, workspaceID string) error
}

// Service manages workspace-to-agent attachments.
type Service struct {
	log   *logger.Logger
	repo  Repository
	mu    sync.RWMutex
	cache map[string]map[string]*WorkspacePermission // agentID -> workspaceID -> Permission
}

// NewService creates a new workspace permissions service.
func NewService(log *logger.Logger, repo Repository) *Service {
	return &Service{
		log:   log,
		repo:  repo,
		cache: make(map[string]map[string]*WorkspacePermission),
	}
}

// SetPermission enables or disables a workspace for an agent.
func (s *Service) SetPermission(ctx context.Context, agentID, workspaceID string, enabled bool) error {
	now := time.Now()
	perm := &WorkspacePermission{
		AgentID:     agentID,
		WorkspaceID: workspaceID,
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.SavePermission(ctx, perm); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache[agentID] == nil {
		s.cache[agentID] = make(map[string]*WorkspacePermission)
	}
	s.cache[agentID][workspaceID] = perm

	return nil
}

// HasPermission checks if an agent has access to a workspace.
func (s *Service) HasPermission(ctx context.Context, agentID, workspaceID string) bool {
	s.mu.RLock()
	if agentPerms, ok := s.cache[agentID]; ok {
		if perm, ok := agentPerms[workspaceID]; ok {
			s.mu.RUnlock()
			return perm.Enabled
		}
	}
	s.mu.RUnlock()

	// Not in cache, check repository
	perm, err := s.repo.GetPermission(ctx, agentID, workspaceID)
	if err != nil || perm == nil {
		return false
	}

	// Update cache
	s.mu.Lock()
	if s.cache[agentID] == nil {
		s.cache[agentID] = make(map[string]*WorkspacePermission)
	}
	s.cache[agentID][workspaceID] = perm
	s.mu.Unlock()

	return perm.Enabled
}

// ListPermissionsByAgent returns all workspace permissions for an agent.
func (s *Service) ListPermissionsByAgent(ctx context.Context, agentID string) ([]*WorkspacePermission, error) {
	return s.repo.ListPermissionsByAgent(ctx, agentID)
}

// GetEnabledWorkspaces returns the IDs of all workspaces enabled for an agent.
func (s *Service) GetEnabledWorkspaces(ctx context.Context, agentID string) ([]string, error) {
	perms, err := s.repo.ListPermissionsByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	var workspaceIDs []string
	for _, perm := range perms {
		if perm.Enabled {
			workspaceIDs = append(workspaceIDs, perm.WorkspaceID)
		}
	}
	return workspaceIDs, nil
}
