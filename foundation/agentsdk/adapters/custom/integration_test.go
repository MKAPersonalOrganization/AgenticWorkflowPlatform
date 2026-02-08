package custom

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/services/permissions"
	"school-with-ai/services/toolregistry"
)

// TestToolFactoryIntegration tests that the tool factory can properly create executable tools
// from tool registry entries based on permissions
func TestToolFactoryIntegration(t *testing.T) {
	ctx := context.Background()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create in-memory repositories
	permissionsRepo := permissions.NewInMemoryRepository()
	toolRegistryRepo := toolregistry.NewInMemoryRepository()

	// Create services
	permissionsService := permissions.NewService(log, permissionsRepo)
	toolRegistry := toolregistry.NewRegistry(log, toolRegistryRepo)

	// Initialize tool registry with default tools
	if err := toolRegistry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize tool registry: %v", err)
	}

	// Initialize default permissions for math instructor
	if err := permissionsService.InitializeDefaultPermissions(ctx, "math-instructor"); err != nil {
		t.Fatalf("Failed to initialize default permissions: %v", err)
	}

	// Get enabled tools for math instructor
	enabledToolIDs, err := permissionsService.GetEnabledTools(ctx, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tools: %v", err)
	}

	if len(enabledToolIDs) == 0 {
		t.Errorf("Expected math instructor to have default tools enabled, but got none")
	}

	t.Logf("Math instructor has %d tools enabled by default", len(enabledToolIDs))

	// Create tool factory
	factory := toolregistry.NewExecutableToolFactory(nil)

	// Verify that we can create executable tools from the enabled tool IDs
	executableToolsCreated := 0
	for _, toolID := range enabledToolIDs {
		tool, err := toolRegistry.GetTool(ctx, toolID)
		if err != nil {
			t.Logf("Skipping tool %s: not found in registry", toolID)
			continue
		}

		executableTool, err := factory.CreateExecutableTool(ctx, tool)
		if err != nil {
			t.Logf("Skipping tool %s: cannot create executable tool: %v", toolID, err)
			continue
		}

		if executableTool == nil {
			t.Logf("Skipping tool %s: created nil executable tool", toolID)
			continue
		}

		// Verify the tool has the expected interface methods
		if executableTool.Name() == "" {
			t.Errorf("Tool %s has empty name", toolID)
		}
		if executableTool.Description() == "" {
			t.Errorf("Tool %s has empty description", toolID)
		}

		executableToolsCreated++
		t.Logf("Successfully created executable tool: %s -> %s", toolID, executableTool.Name())
	}

	if executableToolsCreated == 0 {
		t.Errorf("Expected to create at least one executable tool, but created none")
	}

	t.Logf("Successfully created %d executable tools from %d enabled tool IDs", executableToolsCreated, len(enabledToolIDs))

	// Test permission changes
	t.Log("Testing permission changes...")

	// Disable calc-add tool
	err = permissionsService.SetPermission(ctx, "math-instructor", "calc-add", false)
	if err != nil {
		t.Fatalf("Failed to disable calc-add tool: %v", err)
	}

	// Get enabled tools again
	enabledToolIDsAfterDisable, err := permissionsService.GetEnabledTools(ctx, "math-instructor")
	if err != nil {
		t.Fatalf("Failed to get enabled tools after disable: %v", err)
	}

	// Verify calc-add is not in the list
	for _, toolID := range enabledToolIDsAfterDisable {
		if toolID == "calc-add" {
			t.Errorf("Expected calc-add to be disabled, but it's still in enabled tools list")
		}
	}

	if len(enabledToolIDsAfterDisable) != len(enabledToolIDs)-1 {
		t.Errorf("Expected tool count to decrease by 1 after disabling calc-add, original: %d, after disable: %d",
			len(enabledToolIDs), len(enabledToolIDsAfterDisable))
	}

	t.Logf("After disabling calc-add: math instructor has %d tools enabled", len(enabledToolIDsAfterDisable))

	// Test that other agents are not affected
	err = permissionsService.InitializeDefaultPermissions(ctx, "physics-instructor")
	if err != nil {
		t.Fatalf("Failed to initialize physics instructor permissions: %v", err)
	}

	physicsTools, err := permissionsService.GetEnabledTools(ctx, "physics-instructor")
	if err != nil {
		t.Fatalf("Failed to get physics tools: %v", err)
	}

	// Physics should still have calc-add enabled
	physicsHasCalcAdd := false
	for _, toolID := range physicsTools {
		if toolID == "calc-add" {
			physicsHasCalcAdd = true
			break
		}
	}

	if !physicsHasCalcAdd {
		t.Errorf("Expected physics instructor to still have calc-add enabled")
	}

	t.Logf("Physics instructor has %d tools enabled (including calc-add)", len(physicsTools))
	t.Log("Tool factory integration tests passed!")
}
