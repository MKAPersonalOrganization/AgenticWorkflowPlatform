package permissions

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
)

func TestPermissionsService(t *testing.T) {
	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create service with in-memory repository
	service := NewService(log, NewInMemoryRepository())

	ctx := context.Background()

	// Test setting a permission
	agentID := "test-agent"
	toolID := "calc-power"

	if err := service.SetPermission(ctx, agentID, toolID, true); err != nil {
		t.Fatalf("Failed to set permission: %v", err)
	}

	// Test checking permission
	hasPermission := service.HasPermission(ctx, agentID, toolID)

	if !hasPermission {
		t.Fatal("Expected agent to have permission for tool")
	}

	// Test getting enabled tools
	enabledTools, err := service.GetEnabledTools(ctx, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tools: %v", err)
	}

	found := false
	for _, id := range enabledTools {
		if id == toolID {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Tool should be in enabled tools list")
	}

	// Test disabling permission
	if err := service.SetPermission(ctx, agentID, toolID, false); err != nil {
		t.Fatalf("Failed to disable permission: %v", err)
	}

	// Test checking permission after disabling
	hasPermission = service.HasPermission(ctx, agentID, toolID)

	if hasPermission {
		t.Fatal("Expected agent to not have permission for tool after disabling")
	}

	// Test enabled tools list after disabling
	enabledTools, err = service.GetEnabledTools(ctx, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tools after disabling: %v", err)
	}

	for _, id := range enabledTools {
		if id == toolID {
			t.Fatal("Tool should not be in enabled tools list after disabling")
		}
	}

	t.Log("Permissions service tests passed!")
}

func TestInitializeDefaultPermissions(t *testing.T) {
	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create service with in-memory repository
	service := NewService(log, NewInMemoryRepository())

	ctx := context.Background()

	// Test initializing default permissions for math-instructor
	agentID := "math-instructor"

	if err := service.InitializeDefaultPermissions(ctx, agentID); err != nil {
		t.Fatalf("Failed to initialize default permissions: %v", err)
	}

	// Check that some default tools were enabled
	enabledTools, err := service.GetEnabledTools(ctx, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tools: %v", err)
	}

	if len(enabledTools) == 0 {
		t.Fatal("Expected some default tools to be enabled")
	}

	t.Logf("Math instructor has %d default tools enabled", len(enabledTools))

	// Test that initializing again doesn't overwrite existing permissions
	// First, disable a tool
	if len(enabledTools) > 0 {
		toolToDisable := enabledTools[0]
		if err := service.SetPermission(ctx, agentID, toolToDisable, false); err != nil {
			t.Fatalf("Failed to disable tool: %v", err)
		}

		// Initialize defaults again
		if err := service.InitializeDefaultPermissions(ctx, agentID); err != nil {
			t.Fatalf("Failed to re-initialize default permissions: %v", err)
		}

		// Check that the disabled tool is still disabled
		hasPermission := service.HasPermission(ctx, agentID, toolToDisable)

		if hasPermission {
			t.Fatal("Previously disabled tool should remain disabled after re-initialization")
		}
	}

	t.Log("Default permissions initialization tests passed!")
}

func TestDifferentAgentTypes(t *testing.T) {
	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create service with in-memory repository
	service := NewService(log, NewInMemoryRepository())

	ctx := context.Background()

	// Test different agent types get different default tools
	agentTypes := []string{"math-instructor", "physics-instructor", "chemistry-instructor", "statistics-instructor", "diagramming-instructor"}

	for _, agentType := range agentTypes {
		if err := service.InitializeDefaultPermissions(ctx, agentType); err != nil {
			t.Fatalf("Failed to initialize default permissions for %s: %v", agentType, err)
		}

		enabledTools, err := service.GetEnabledTools(ctx, agentType)
		if err != nil {
			t.Fatalf("Failed to get enabled tools for %s: %v", agentType, err)
		}

		t.Logf("%s has %d default tools enabled", agentType, len(enabledTools))

		if len(enabledTools) == 0 {
			t.Fatalf("Expected %s to have some default tools", agentType)
		}
	}

	t.Log("Different agent types tests passed!")
}

func TestPermissionOverridesBehavior(t *testing.T) {
	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create service with in-memory repository
	service := NewService(log, NewInMemoryRepository())

	ctx := context.Background()

	// Initialize default permissions for math instructor
	agentID := "math-instructor"
	if err := service.InitializeDefaultPermissions(ctx, agentID); err != nil {
		t.Fatalf("Failed to initialize default permissions: %v", err)
	}

	// Get initial enabled tools (should be all defaults)
	initialTools, err := service.GetEnabledTools(ctx, agentID)
	if err != nil {
		t.Fatalf("Failed to get initial enabled tools: %v", err)
	}

	if len(initialTools) == 0 {
		t.Fatal("Expected some default tools to be enabled initially")
	}

	initialCount := len(initialTools)
	t.Logf("Math instructor initially has %d tools enabled", initialCount)

	// Disable one tool
	toolToDisable := "calc-add"
	if err := service.SetPermission(ctx, agentID, toolToDisable, false); err != nil {
		t.Fatalf("Failed to disable tool: %v", err)
	}

	// Check enabled tools after disabling one
	afterDisableTools, err := service.GetEnabledTools(ctx, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tools after disable: %v", err)
	}

	// Should have one fewer tool
	if len(afterDisableTools) != initialCount-1 {
		t.Fatalf("Expected %d tools after disabling one, got %d", initialCount-1, len(afterDisableTools))
	}

	// The disabled tool should not be in the list
	for _, toolID := range afterDisableTools {
		if toolID == toolToDisable {
			t.Fatalf("Disabled tool %s should not be in enabled tools list", toolToDisable)
		}
	}

	// Enable a tool that wasn't in defaults
	newTool := "stats-mean"
	if err := service.SetPermission(ctx, agentID, newTool, true); err != nil {
		t.Fatalf("Failed to enable new tool: %v", err)
	}

	// Check enabled tools after enabling new tool
	afterEnableTools, err := service.GetEnabledTools(ctx, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tools after enable: %v", err)
	}

	// Should have same count as initial (one disabled, one new enabled)
	if len(afterEnableTools) != initialCount {
		t.Fatalf("Expected %d tools after enable/disable operations, got %d", initialCount, len(afterEnableTools))
	}

	// The new tool should be in the list
	found := false
	for _, toolID := range afterEnableTools {
		if toolID == newTool {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("Newly enabled tool %s should be in enabled tools list", newTool)
	}

	// The disabled tool should still not be in the list
	for _, toolID := range afterEnableTools {
		if toolID == toolToDisable {
			t.Fatalf("Disabled tool %s should still not be in enabled tools list", toolToDisable)
		}
	}

	t.Log("Permission overrides behavior tests passed!")
}
