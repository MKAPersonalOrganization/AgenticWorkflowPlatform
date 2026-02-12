package edgequakepermissions

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"

	"github.com/google/uuid"
)

// Service provides logic for managing Edgequake-mirroring permissions.
type Service struct {
	log  *logger.Logger
	repo domain.EdgequakePermissionRepository
}

// NewService creates a new PermissionsService.
func NewService(log *logger.Logger, repo domain.EdgequakePermissionRepository) *Service {
	return &Service{
		log:  log,
		repo: repo,
	}
}

// GetTenantContext resolves the active tenant and workspace for a user.
// This is used to populate headers for Edgequake API calls.
func (s *Service) GetTenantContext(ctx context.Context, userID string) (*domain.TenantContext, error) {
	memberships, err := s.repo.ListMembershipsByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships: %w", err)
	}

	if len(memberships) == 0 {
		return nil, fmt.Errorf("no memberships found for user %s", userID)
	}

	// For now, we take the first membership. In a real scenario, the user might
	// have an "active" workspace/tenant selected in the UI.
	m := memberships[0]

	tenant, err := s.repo.GetTenant(m.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	context := &domain.TenantContext{
		TenantID: &tenant.ID,
		Role:     string(m.Role),
		UserID:   userID,
	}

	if m.WorkspaceID != nil {
		workspace, err := s.repo.GetWorkspace(*m.WorkspaceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace: %w", err)
		}
		context.WorkspaceID = &workspace.ID
	}

	return context, nil
}

// CreateTenant creates a new tenant in the registry.
func (s *Service) CreateTenant(ctx context.Context, name, slug string) (*domain.EdgequakeTenant, error) {
	now := time.Now().UTC()
	tenant := &domain.EdgequakeTenant{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Default values matching Edgequake's Pro plan as per user previous context
	tenant.Plan = "pro"
	tenant.MaxWorkspaces = 500
	tenant.MaxUsers = 50

	if err := s.repo.SaveTenant(tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

// CreateWorkspace creates a new workspace within a tenant.
func (s *Service) CreateWorkspace(ctx context.Context, tenantID uuid.UUID, name, slug string) (*domain.EdgequakeWorkspace, error) {
	now := time.Now().UTC()
	workspace := &domain.EdgequakeWorkspace{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      name,
		Slug:      slug,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.SaveWorkspace(workspace); err != nil {
		return nil, err
	}
	return workspace, nil
}

// AddMember adds a user to a tenant/workspace with a specific role.
func (s *Service) AddMember(ctx context.Context, userID string, tenantID uuid.UUID, workspaceID *uuid.UUID, role domain.EdgequakeRole) error {
	membership := &domain.EdgequakeMembership{
		ID:          uuid.New(),
		UserID:      userID,
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
		Role:        role,
		IsActive:    true,
		JoinedAt:    time.Now().UTC(),
	}

	return s.repo.SaveMembership(membership)
}
