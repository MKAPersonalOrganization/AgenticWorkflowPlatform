package workspace

import (
	"context"
	"school-with-ai/foundation/logger"
	"sync"
	"time"
)

// WorkspacePermission represents a workspace attached to an agent.
type WorkspacePermission struct {
	UserID      string    `json:"user_id"`
	AgentID     string    `json:"agent_id"`
	WorkspaceID string    `json:"workspace_id"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repository defines the storage interface for workspace permissions.
type Repository interface {
	GetPermission(ctx context.Context, userID, agentID, workspaceID string) (*WorkspacePermission, error)
	ListPermissionsByAgent(ctx context.Context, userID, agentID string) ([]*WorkspacePermission, error)
	SavePermission(ctx context.Context, perm *WorkspacePermission) error
	DeletePermission(ctx context.Context, userID, agentID, workspaceID string) error
}

// Service manages workspace-to-agent attachments.
type Service struct {
	log   *logger.Logger
	repo  Repository
	mu    sync.RWMutex
	cache map[string]map[string]map[string]*WorkspacePermission // userID -> agentID -> workspaceID -> Permission
}

const wsCacheMaxUsers = 5000 // Max distinct user entries in the cache

// NewService creates a new workspace permissions service.
func NewService(log *logger.Logger, repo Repository) *Service {
	return &Service{
		log:   log,
		repo:  repo,
		cache: make(map[string]map[string]map[string]*WorkspacePermission),
	}
}

// evictCacheIfNeeded evicts user entries when the cache exceeds its size limit.
// Must be called while holding s.mu (write lock).
func (s *Service) evictCacheIfNeeded() {
	if len(s.cache) >= wsCacheMaxUsers {
		count := 0
		half := len(s.cache) / 2
		for k := range s.cache {
			delete(s.cache, k)
			count++
			if count >= half {
				break
			}
		}
	}
}

// SetPermission enables or disables a workspace for an agent.
func (s *Service) SetPermission(ctx context.Context, userID, agentID, workspaceID string, enabled bool) error {
	now := time.Now()
	perm := &WorkspacePermission{
		UserID:      userID,
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
	s.evictCacheIfNeeded()
	if _, ok := s.cache[userID]; !ok {
		s.cache[userID] = make(map[string]map[string]*WorkspacePermission)
	}
	if _, ok := s.cache[userID][agentID]; !ok {
		s.cache[userID][agentID] = make(map[string]*WorkspacePermission)
	}
	s.cache[userID][agentID][workspaceID] = perm

	return nil
}

// HasPermission checks if an agent has access to a workspace.
func (s *Service) HasPermission(ctx context.Context, userID, agentID, workspaceID string) bool {
	s.mu.RLock()
	if userPerms, ok := s.cache[userID]; ok {
		if agentPerms, ok := userPerms[agentID]; ok {
			if perm, ok := agentPerms[workspaceID]; ok {
				s.mu.RUnlock()
				return perm.Enabled
			}
		}
	}
	s.mu.RUnlock()

	// Not in cache, check repository
	perm, err := s.repo.GetPermission(ctx, userID, agentID, workspaceID)
	if err != nil || perm == nil {
		return false
	}

	// Update cache
	s.mu.Lock()
	s.evictCacheIfNeeded()
	if _, ok := s.cache[userID]; !ok {
		s.cache[userID] = make(map[string]map[string]*WorkspacePermission)
	}
	if _, ok := s.cache[userID][agentID]; !ok {
		s.cache[userID][agentID] = make(map[string]*WorkspacePermission)
	}
	s.cache[userID][agentID][workspaceID] = perm
	s.mu.Unlock()

	return perm.Enabled
}

// ListPermissionsByAgent returns all workspace permissions for an agent.
func (s *Service) ListPermissionsByAgent(ctx context.Context, userID, agentID string) ([]*WorkspacePermission, error) {
	return s.repo.ListPermissionsByAgent(ctx, userID, agentID)
}

// GetEnabledWorkspaces returns the IDs of all workspaces enabled for an agent.
func (s *Service) GetEnabledWorkspaces(ctx context.Context, userID, agentID string) ([]string, error) {
	perms, err := s.repo.ListPermissionsByAgent(ctx, userID, agentID)
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
