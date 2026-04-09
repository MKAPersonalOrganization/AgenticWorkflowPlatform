package workspace

import (
	"aicompass/foundation/logger"
	"aicompass/foundation/permcache"
	"context"
	"time"

	"github.com/redis/go-redis/v9"
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
	cache *permcache.Cache[WorkspacePermission]
}

// NewService creates a new workspace permissions service.
// The rdb client is used for distributed Redis caching of permission lookups.
func NewService(log *logger.Logger, repo Repository, rdb redis.Cmdable) *Service {
	return &Service{
		log:   log,
		repo:  repo,
		cache: permcache.New[WorkspacePermission](rdb, "workspaceperm"),
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

	s.cache.Set(userID, perm, agentID, workspaceID)
	return nil
}

// HasPermission checks if an agent has access to a workspace.
func (s *Service) HasPermission(ctx context.Context, userID, agentID, workspaceID string) bool {
	if cached, ok := s.cache.Get(userID, agentID, workspaceID); ok {
		return cached.Enabled
	}

	// Not in cache, check repository
	perm, err := s.repo.GetPermission(ctx, userID, agentID, workspaceID)
	if err != nil || perm == nil {
		return false
	}

	s.cache.Set(userID, perm, agentID, workspaceID)
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
