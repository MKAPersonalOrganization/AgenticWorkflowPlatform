package permissions_test

import (
	"context"
	"os"
	"school-with-ai/foundation/logger"
	"school-with-ai/platform/permissions"
	"school-with-ai/platform/permissions/mocks"
	"school-with-ai/platform/toolregistry"
	"testing"

	toolregistry_mocks "school-with-ai/platform/toolregistry/mocks"
)

func TestToolSetPermissions(t *testing.T) {
	ctx := context.Background()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create repositories and services
	permissionsRepo := mocks.NewInMemoryRepository()
	toolRegistryRepo := toolregistry_mocks.NewInMemoryRepository()

	permissionsService := permissions.NewService(log, permissionsRepo)
	toolRegistry := toolregistry.NewRegistry(log, toolRegistryRepo)

	// Initialize tool registry
	if err := toolRegistry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize tool registry: %v", err)
	}

	userID := "test-user"

	// Test: Agents start with no default tool sets (decoupled architecture)
	agentTypes := []string{"math-instructor", "statistics-instructor"}

	for _, agentType := range agentTypes {
		t.Logf("Testing no defaults for %s", agentType)

		enabledToolSets, err := permissionsService.GetEnabledToolSets(ctx, userID, agentType)
		if err != nil {
			t.Fatalf("Failed to get enabled tool sets for %s: %v", agentType, err)
		}

		if len(enabledToolSets) != 0 {
			t.Errorf("Expected %s to have no default tool sets, but got %d: %v", agentType, len(enabledToolSets), enabledToolSets)
		}
	}

	// Test: Explicitly enable tool sets for math-instructor
	t.Log("Testing explicit tool set permission changes...")

	err := permissionsService.SetToolSetPermission(ctx, userID, "math-instructor", "calculator-set", true)
	if err != nil {
		t.Fatalf("Failed to enable calculator-set for math-instructor: %v", err)
	}

	err = permissionsService.SetToolSetPermission(ctx, userID, "math-instructor", "cognitive-set", true)
	if err != nil {
		t.Fatalf("Failed to enable cognitive-set for math-instructor: %v", err)
	}

	// Verify math-instructor has the explicitly enabled tool sets
	mathToolSets, err := permissionsService.GetEnabledToolSets(ctx, userID, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for math-instructor: %v", err)
	}

	if len(mathToolSets) != 2 {
		t.Errorf("Expected math-instructor to have 2 tool sets, got %d: %v", len(mathToolSets), mathToolSets)
	}

	// Test: Disable a tool set
	err = permissionsService.SetToolSetPermission(ctx, userID, "math-instructor", "calculator-set", false)
	if err != nil {
		t.Fatalf("Failed to disable calculator-set for math-instructor: %v", err)
	}

	mathToolSets, err = permissionsService.GetEnabledToolSets(ctx, userID, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for math-instructor: %v", err)
	}

	for _, toolSetID := range mathToolSets {
		if toolSetID == "calculator-set" {
			t.Errorf("Expected calculator-set to be disabled for math-instructor, but it's still enabled")
		}
	}

	if len(mathToolSets) != 1 {
		t.Errorf("Expected math-instructor to have 1 tool set after disabling, got %d: %v", len(mathToolSets), mathToolSets)
	}

	t.Logf("After disabling calculator-set, math-instructor has %d tool sets: %v", len(mathToolSets), mathToolSets)

	// Test: Other agents remain unaffected (isolated per agent)
	physicsToolSets, err := permissionsService.GetEnabledToolSets(ctx, userID, "physics-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for physics-instructor: %v", err)
	}

	if len(physicsToolSets) != 0 {
		t.Errorf("Expected physics-instructor to have no tool sets (nothing configured), got %d: %v", len(physicsToolSets), physicsToolSets)
	}

	// Test: Enable a new tool set
	err = permissionsService.SetToolSetPermission(ctx, userID, "math-instructor", "chemistry-set", true)
	if err != nil {
		t.Fatalf("Failed to enable chemistry-set for math-instructor: %v", err)
	}

	mathToolSetsAfterEnable, err := permissionsService.GetEnabledToolSets(ctx, userID, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets after enabling chemistry-set: %v", err)
	}

	chemistrySetEnabled := false
	for _, toolSetID := range mathToolSetsAfterEnable {
		if toolSetID == "chemistry-set" {
			chemistrySetEnabled = true
			break
		}
	}

	if !chemistrySetEnabled {
		t.Errorf("Expected chemistry-set to be enabled for math-instructor after manual enable")
	}

	t.Logf("After enabling chemistry-set, math-instructor has %d tool sets: %v", len(mathToolSetsAfterEnable), mathToolSetsAfterEnable)

	// Test: Individual tool set permission retrieval
	calcPerm, err := permissionsService.GetToolSetPermission(ctx, userID, "math-instructor", "calculator-set")
	if err != nil {
		t.Fatalf("Failed to get calculator-set permission: %v", err)
	}

	if calcPerm.Enabled {
		t.Errorf("Expected calculator-set permission to be disabled, but it's enabled")
	}

	// Test: HasToolSetPermission checks
	if permissionsService.HasToolSetPermission(ctx, userID, "math-instructor", "calculator-set") {
		t.Errorf("Expected math-instructor to NOT have calculator-set permission")
	}

	if permissionsService.HasToolSetPermission(ctx, userID, "physics-instructor", "calculator-set") {
		t.Errorf("Expected physics-instructor to NOT have calculator-set permission (nothing configured)")
	}

	if !permissionsService.HasToolSetPermission(ctx, userID, "math-instructor", "chemistry-set") {
		t.Errorf("Expected math-instructor to have chemistry-set permission after manual enable")
	}

	t.Log("Tool set permissions tests passed!")
}
