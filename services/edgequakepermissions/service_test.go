package edgequakepermissions

import (
	"context"
	"os"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
)

func TestEdgequakePermissions(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	repo := NewInMemoryRepository()
	service := NewService(log, repo)
	ctx := context.Background()

	userID := "user-123"

	// 1. Create Tenant
	tenant, err := service.CreateTenant(ctx, "Test School", "test-school")
	if err != nil {
		t.Fatalf("Failed to create tenant: %v", err)
	}

	// 2. Create Workspace
	workspace, err := service.CreateWorkspace(ctx, tenant.ID, "Physics Lab", "physics")
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// 3. Add Member
	err = service.AddMember(ctx, userID, tenant.ID, &workspace.ID, domain.RoleAdmin)
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	// 4. Resolve Context
	tCtx, err := service.GetTenantContext(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to resolve tenant context: %v", err)
	}

	if *tCtx.TenantID != tenant.ID {
		t.Errorf("Expected tenant ID %v, got %v", tenant.ID, *tCtx.TenantID)
	}

	if *tCtx.WorkspaceID != workspace.ID {
		t.Errorf("Expected workspace ID %v, got %v", workspace.ID, *tCtx.WorkspaceID)
	}

	if tCtx.Role != string(domain.RoleAdmin) {
		t.Errorf("Expected role %s, got %s", domain.RoleAdmin, tCtx.Role)
	}

	if tCtx.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, tCtx.UserID)
	}
}

func TestResolveContextNoMembership(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	repo := NewInMemoryRepository()
	service := NewService(log, repo)
	ctx := context.Background()

	_, err := service.GetTenantContext(ctx, "unknown-user")
	if err == nil {
		t.Fatal("Expected error when resolving context for user with no memberships")
	}
}

func TestResolveContextNoWorkspace(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	repo := NewInMemoryRepository()
	service := NewService(log, repo)
	ctx := context.Background()

	userID := "user-456"
	tenant, _ := service.CreateTenant(ctx, "School B", "school-b")

	// Add membership without workspace
	err := service.AddMember(ctx, userID, tenant.ID, nil, domain.RoleUser)
	if err != nil {
		t.Fatalf("Failed to add member: %v", err)
	}

	tCtx, err := service.GetTenantContext(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to resolve context: %v", err)
	}

	if tCtx.WorkspaceID != nil {
		t.Errorf("Expected nil workspace ID, got %v", tCtx.WorkspaceID)
	}

	if tCtx.Role != string(domain.RoleUser) {
		t.Errorf("Expected role %s, got %s", domain.RoleUser, tCtx.Role)
	}
}

func TestResolveContextMultipleMemberships(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	repo := NewInMemoryRepository()
	service := NewService(log, repo)
	ctx := context.Background()

	userID := "multi-user"

	// 1. Join Tenant A
	tenantA, _ := service.CreateTenant(ctx, "School A", "school-a")
	wsA, _ := service.CreateWorkspace(ctx, tenantA.ID, "WS A", "ws-a")
	_ = service.AddMember(ctx, userID, tenantA.ID, &wsA.ID, domain.RoleUser)

	// 2. Join Tenant B
	tenantB, _ := service.CreateTenant(ctx, "School B", "school-b")
	wsB, _ := service.CreateWorkspace(ctx, tenantB.ID, "WS B", "ws-b")
	_ = service.AddMember(ctx, userID, tenantB.ID, &wsB.ID, domain.RoleAdmin)

	// 3. Resolve context
	tCtx, err := service.GetTenantContext(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to resolve context: %v", err)
	}

	// Currently implementation takes memberships[0]
	// In memory repo appends to slice, so first one added is memberships[0]
	if *tCtx.TenantID != tenantA.ID {
		t.Errorf("Expected tenant A %v, got %v", tenantA.ID, *tCtx.TenantID)
	}

	if tCtx.Role != string(domain.RoleUser) {
		t.Errorf("Expected role %s, got %s", domain.RoleUser, tCtx.Role)
	}
}
