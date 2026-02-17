package permissions_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/services/permissions"
	"school-with-ai/services/toolregistry"
)

func TestToolSetPermissions(t *testing.T) {
	ctx := context.Background()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create repositories and services
	permissionsRepo := permissions.NewInMemoryRepository()
	toolRegistryRepo := toolregistry.NewInMemoryRepository()

	permissionsService := permissions.NewService(log, permissionsRepo)
	toolRegistry := toolregistry.NewRegistry(log, toolRegistryRepo)

	// Initialize tool registry
	if err := toolRegistry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize tool registry: %v", err)
	}

	// Test: Get default tool sets for different agents
	agentTypes := []string{"math-instructor", "statistics-instructor"}

	for _, agentType := range agentTypes {
		t.Logf("Testing default tool sets for %s", agentType)

		enabledToolSets, err := permissionsService.GetEnabledToolSets(ctx, agentType)
		if err != nil {
			t.Fatalf("Failed to get enabled tool sets for %s: %v", agentType, err)
		}

		if len(enabledToolSets) == 0 {
			t.Errorf("Expected %s to have default tool sets, but got none", agentType)
		}

		t.Logf("%s has %d default tool sets: %v", agentType, len(enabledToolSets), enabledToolSets)
	}

	// Test: Tool set permission changes
	t.Log("Testing tool set permission changes...")

	// Disable calculator-set for math-instructor
	err := permissionsService.SetToolSetPermission(ctx, "math-instructor", "calculator-set", false)
	if err != nil {
		t.Fatalf("Failed to disable calculator-set for math-instructor: %v", err)
	}

	// Verify calculator-set is disabled for math-instructor
	mathToolSets, err := permissionsService.GetEnabledToolSets(ctx, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for math-instructor: %v", err)
	}

	for _, toolSetID := range mathToolSets {
		if toolSetID == "calculator-set" {
			t.Errorf("Expected calculator-set to be disabled for math-instructor, but it's still enabled")
		}
	}

	t.Logf("After disabling calculator-set, math-instructor has %d tool sets: %v", len(mathToolSets), mathToolSets)

	// Verify other agents are unaffected
	physicsToolSets, err := permissionsService.GetEnabledToolSets(ctx, "physics-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for physics-instructor: %v", err)
	}

	physicsHasCalculator := false
	for _, toolSetID := range physicsToolSets {
		if toolSetID == "calculator-set" {
			physicsHasCalculator = true
			break
		}
	}

	if !physicsHasCalculator {
		t.Errorf("Expected physics-instructor to still have calculator-set enabled")
	}

	// Test: Enable a tool set that's not in defaults
	err = permissionsService.SetToolSetPermission(ctx, "math-instructor", "chemistry-set", true)
	if err != nil {
		t.Fatalf("Failed to enable chemistry-set for math-instructor: %v", err)
	}

	mathToolSetsAfterEnable, err := permissionsService.GetEnabledToolSets(ctx, "math-instructor")
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
	calcPerm, err := permissionsService.GetToolSetPermission(ctx, "math-instructor", "calculator-set")
	if err != nil {
		t.Fatalf("Failed to get calculator-set permission: %v", err)
	}

	if calcPerm.Enabled {
		t.Errorf("Expected calculator-set permission to be disabled, but it's enabled")
	}

	// Test: Check if agent has permission for tool set
	if permissionsService.HasToolSetPermission(ctx, "math-instructor", "calculator-set") {
		t.Errorf("Expected math-instructor to NOT have calculator-set permission")
	}

	if !permissionsService.HasToolSetPermission(ctx, "physics-instructor", "calculator-set") {
		t.Errorf("Expected physics-instructor to have calculator-set permission")
	}

	if !permissionsService.HasToolSetPermission(ctx, "math-instructor", "chemistry-set") {
		t.Errorf("Expected math-instructor to have chemistry-set permission after manual enable")
	}

	t.Log("Tool set permissions tests passed!")
}
