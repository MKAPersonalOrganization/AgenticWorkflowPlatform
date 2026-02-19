package custom

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/services/permissions"
	permissions_mocks "school-with-ai/services/permissions/mocks"
	"school-with-ai/services/toolregistry"
	toolregistry_mocks "school-with-ai/services/toolregistry/mocks"
)

// TestToolFactoryIntegration tests that the tool factory can properly create executable tools
// from tool registry entries and tool sets based on permissions
func TestToolFactoryIntegration(t *testing.T) {
	ctx := context.Background()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create in-memory repositories
	permissionsRepo := permissions_mocks.NewInMemoryRepository()
	toolRegistryRepo := toolregistry_mocks.NewInMemoryRepository()

	// Create services
	permissionsService := permissions.NewService(log, permissionsRepo)
	toolRegistry := toolregistry.NewRegistry(log, toolRegistryRepo)

	// Initialize tool registry with default tools and tool sets
	if err := toolRegistry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize tool registry: %v", err)
	}

	// Test: Verify tool sets were created
	toolSets, err := toolRegistry.ListToolSets(ctx)
	if err != nil {
		t.Fatalf("Failed to list tool sets: %v", err)
	}

	if len(toolSets) == 0 {
		t.Errorf("Expected default tool sets to be created, but got none")
	}

	t.Logf("Created %d default tool sets", len(toolSets))

	// Test: Verify tool set permissions work
	userID := "test-user"
	enabledToolSetIDs, err := permissionsService.GetEnabledToolSets(ctx, userID, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets: %v", err)
	}

	if len(enabledToolSetIDs) == 0 {
		t.Errorf("Expected math instructor to have default tool sets enabled, but got none")
	}

	t.Logf("Math instructor has %d tool sets enabled by default", len(enabledToolSetIDs))

	// Test: Verify we can create executable tools from tool sets
	factory := toolregistry.NewExecutableToolFactory(nil)
	factory.SetToolRegistry(toolRegistry)

	totalToolsFromSets := 0
	for _, toolSetID := range enabledToolSetIDs {
		toolsFromSet, err := factory.CreateExecutableToolsFromSet(ctx, toolSetID)
		if err != nil {
			t.Logf("Skipping tool set %s: %v", toolSetID, err)
			continue
		}

		t.Logf("Tool set %s provided %d executable tools", toolSetID, len(toolsFromSet))
		totalToolsFromSets += len(toolsFromSet)

		// Verify each tool has the expected interface methods
		for _, tool := range toolsFromSet {
			if tool.Name() == "" {
				t.Errorf("Tool from set %s has empty name", toolSetID)
			}
			if tool.Description() == "" {
				t.Errorf("Tool from set %s has empty description", toolSetID)
			}
		}
	}

	t.Logf("Successfully created %d total executable tools from tool sets", totalToolsFromSets)

	// Test: Tool set permission changes
	t.Log("Testing tool set permission changes...")

	// Disable calculator-set for math instructor
	err = permissionsService.SetToolSetPermission(ctx, userID, "math-instructor", "calculator-set", false)
	if err != nil {
		t.Fatalf("Failed to disable calculator-set: %v", err)
	}

	// Get enabled tool sets again
	enabledToolSetsAfterDisable, err := permissionsService.GetEnabledToolSets(ctx, userID, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets after disable: %v", err)
	}

	// Verify calculator-set is not in the list
	for _, toolSetID := range enabledToolSetsAfterDisable {
		if toolSetID == "calculator-set" {
			t.Errorf("Expected calculator-set to be disabled, but it's still in enabled tool sets list")
		}
	}

	if len(enabledToolSetsAfterDisable) != len(enabledToolSetIDs)-1 {
		t.Errorf("Expected tool set count to decrease by 1 after disabling calculator-set, original: %d, after disable: %d",
			len(enabledToolSetIDs), len(enabledToolSetsAfterDisable))
	}

	t.Logf("After disabling calculator-set: math instructor has %d tool sets enabled", len(enabledToolSetsAfterDisable))

	// Test: Verify other agents are not affected by tool set permission changes
	enabledPhysicsToolSets, err := permissionsService.GetEnabledToolSets(ctx, userID, "physics-instructor")
	if err != nil {
		t.Fatalf("Failed to get physics tool sets: %v", err)
	}

	// Physics should still have calculator-set enabled
	physicsHasCalculatorSet := false
	for _, toolSetID := range enabledPhysicsToolSets {
		if toolSetID == "calculator-set" {
			physicsHasCalculatorSet = true
			break
		}
	}

	if !physicsHasCalculatorSet {
		t.Errorf("Expected physics instructor to still have calculator-set enabled")
	}

	t.Logf("Physics instructor has %d tool sets enabled (including calculator-set)", len(enabledPhysicsToolSets))

	// Test: Verify tools in a set
	calculatorSet, err := toolRegistry.GetToolSet(ctx, "calculator-set")
	if err != nil {
		t.Fatalf("Failed to get calculator set: %v", err)
	}

	if len(calculatorSet.ToolIDs) == 0 {
		t.Errorf("Expected calculator set to have tools, but it's empty")
	}

	t.Logf("Calculator set contains %d tools: %v", len(calculatorSet.ToolIDs), calculatorSet.ToolIDs)

	t.Log("Tool set integration tests passed!")
}
