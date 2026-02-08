package permissions

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
)

func TestIndividualToolPermissions(t *testing.T) {
	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create service with in-memory repository
	service := NewService(log, NewInMemoryRepository())

	ctx := context.Background()

	agentID := "math-instructor"
	toolSetID := "calculator-set"
	toolID := "calc-add"

	// Test: Set individual tool permission
	err := service.SetPermission(ctx, agentID, toolSetID, toolID, true)
	if err != nil {
		t.Fatalf("Failed to set individual tool permission: %v", err)
	}

	// Test: Check individual tool permission
	hasPermission := service.HasPermission(ctx, agentID, toolSetID, toolID)
	if !hasPermission {
		t.Fatal("Expected agent to have permission for individual tool")
	}

	// Test: Get permission details
	permission, err := service.GetPermission(ctx, agentID, toolSetID, toolID)
	if err != nil {
		t.Fatalf("Failed to get individual tool permission: %v", err)
	}

	if permission.AgentID != agentID {
		t.Errorf("Expected agent ID %s, got %s", agentID, permission.AgentID)
	}

	if permission.ToolSetID != toolSetID {
		t.Errorf("Expected tool set ID %s, got %s", toolSetID, permission.ToolSetID)
	}

	if permission.ToolID != toolID {
		t.Errorf("Expected tool ID %s, got %s", toolID, permission.ToolID)
	}

	if !permission.Enabled {
		t.Error("Permission should be enabled")
	}
}

func TestIndividualToolPermissionInheritance(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	service := NewService(log, NewInMemoryRepository())
	ctx := context.Background()

	agentID := "math-instructor"
	toolSetID := "calculator-set"
	toolID := "calc-multiply"

	// Test: Tool inherits permission from tool set when no explicit permission is set

	// First, enable the tool set
	err := service.SetToolSetPermission(ctx, agentID, toolSetID, true)
	if err != nil {
		t.Fatalf("Failed to enable tool set: %v", err)
	}

	// Check if individual tool inherits permission from tool set (should be true)
	hasPermission := service.HasPermission(ctx, agentID, toolSetID, toolID)
	if !hasPermission {
		t.Fatal("Tool should inherit permission from enabled tool set")
	}

	// Now disable the tool set
	err = service.SetToolSetPermission(ctx, agentID, toolSetID, false)
	if err != nil {
		t.Fatalf("Failed to disable tool set: %v", err)
	}

	// Check if individual tool inherits disabled permission from tool set
	hasPermission = service.HasPermission(ctx, agentID, toolSetID, toolID)
	if hasPermission {
		t.Fatal("Tool should inherit disabled permission from tool set")
	}
}

func TestIndividualToolPermissionOverride(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	service := NewService(log, NewInMemoryRepository())
	ctx := context.Background()

	agentID := "math-instructor"
	toolSetID := "calculator-set"
	toolID := "calc-divide"

	// Test: Individual tool permission overrides tool set permission

	// First, disable the tool set
	err := service.SetToolSetPermission(ctx, agentID, toolSetID, false)
	if err != nil {
		t.Fatalf("Failed to disable tool set: %v", err)
	}

	// Explicitly enable the individual tool
	err = service.SetPermission(ctx, agentID, toolSetID, toolID, true)
	if err != nil {
		t.Fatalf("Failed to enable individual tool: %v", err)
	}

	// The individual tool should be enabled despite the tool set being disabled
	hasPermission := service.HasPermission(ctx, agentID, toolSetID, toolID)
	if !hasPermission {
		t.Fatal("Individual tool permission should override tool set permission")
	}

	// Check tool set is still disabled
	hasToolSetPermission := service.HasToolSetPermission(ctx, agentID, toolSetID)
	if hasToolSetPermission {
		t.Fatal("Tool set should remain disabled")
	}
}

func TestGetEnabledToolsInSet(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	service := NewService(log, NewInMemoryRepository())
	ctx := context.Background()

	agentID := "math-instructor"
	toolSetID := "calculator-set"

	// Test: Get enabled tools in a disabled tool set (should return empty)
	err := service.SetToolSetPermission(ctx, agentID, toolSetID, false)
	if err != nil {
		t.Fatalf("Failed to disable tool set: %v", err)
	}

	enabledTools, err := service.GetEnabledToolsInSet(ctx, agentID, toolSetID)
	if err != nil {
		t.Fatalf("Failed to get enabled tools in set: %v", err)
	}

	if len(enabledTools) != 0 {
		t.Errorf("Expected 0 enabled tools in disabled tool set, got %d", len(enabledTools))
	}

	// Test: Enable tool set and add individual tool permissions
	err = service.SetToolSetPermission(ctx, agentID, toolSetID, true)
	if err != nil {
		t.Fatalf("Failed to enable tool set: %v", err)
	}

	// Enable specific tools individually
	tools := []string{"calc-add", "calc-multiply"}
	for _, toolID := range tools {
		err = service.SetPermission(ctx, agentID, toolSetID, toolID, true)
		if err != nil {
			t.Fatalf("Failed to enable tool %s: %v", toolID, err)
		}
	}

	// Disable one tool
	err = service.SetPermission(ctx, agentID, toolSetID, "calc-subtract", false)
	if err != nil {
		t.Fatalf("Failed to disable tool calc-subtract: %v", err)
	}

	enabledTools, err = service.GetEnabledToolsInSet(ctx, agentID, toolSetID)
	if err != nil {
		t.Fatalf("Failed to get enabled tools in set: %v", err)
	}

	// Should have 2 enabled tools
	if len(enabledTools) != 2 {
		t.Errorf("Expected 2 enabled tools, got %d: %v", len(enabledTools), enabledTools)
	}

	// Check specific tools are in the list
	expectedTools := map[string]bool{"calc-add": true, "calc-multiply": true}
	for _, toolID := range enabledTools {
		if !expectedTools[toolID] {
			t.Errorf("Unexpected enabled tool: %s", toolID)
		}
		delete(expectedTools, toolID)
	}

	if len(expectedTools) > 0 {
		t.Errorf("Missing expected enabled tools: %v", expectedTools)
	}
}
