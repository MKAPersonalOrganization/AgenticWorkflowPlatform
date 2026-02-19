package permissions_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/services/permissions"
	"school-with-ai/services/permissions/mocks"
)

func TestToolSetPermissionsService(t *testing.T) {
	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create service with in-memory repository
	service := permissions.NewService(log, mocks.NewInMemoryRepository())

	ctx := context.Background()

	userID := "test-user"

	// Test setting a tool set permission
	agentID := "math-instructor"
	toolSetID := "calculator-set"

	if err := service.SetToolSetPermission(ctx, userID, agentID, toolSetID, true); err != nil {
		t.Fatalf("Failed to set tool set permission: %v", err)
	}

	// Test checking tool set permission
	hasPermission := service.HasToolSetPermission(ctx, userID, agentID, toolSetID)
	if !hasPermission {
		t.Fatal("Expected agent to have permission for tool set")
	}

	// Test getting enabled tool sets
	enabledToolSets, err := service.GetEnabledToolSets(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets: %v", err)
	}

	found := false
	for _, id := range enabledToolSets {
		if id == toolSetID {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("Tool set should be in enabled tool sets list")
	}
}

func TestToolSetDisabling(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	service := permissions.NewService(log, mocks.NewInMemoryRepository())
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-instructor"
	toolSetID := "calculator-set"

	// First enable the tool set
	if err := service.SetToolSetPermission(ctx, userID, agentID, toolSetID, true); err != nil {
		t.Fatalf("Failed to enable tool set permission: %v", err)
	}

	// Then disable it
	if err := service.SetToolSetPermission(ctx, userID, agentID, toolSetID, false); err != nil {
		t.Fatalf("Failed to disable tool set permission: %v", err)
	}

	// Verify it's disabled
	hasPermission := service.HasToolSetPermission(ctx, userID, agentID, toolSetID)
	if hasPermission {
		t.Fatal("Expected agent to NOT have permission for disabled tool set")
	}

	// Check enabled tool sets don't include the disabled one
	enabledToolSets, err := service.GetEnabledToolSets(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets: %v", err)
	}

	for _, id := range enabledToolSets {
		if id == toolSetID {
			t.Fatal("Disabled tool set should not be in enabled tool sets list")
		}
	}
}

func TestDefaultToolSets(t *testing.T) {
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	service := permissions.NewService(log, mocks.NewInMemoryRepository())
	ctx := context.Background()

	userID := "test-user"

	// Test different agent types have default tool sets
	testCases := []struct {
		agentID  string
		expected []string
	}{
		{"math-instructor", []string{"calculator-set", "visualization-set", "memory-set", "document-set", "knowledge-base-set"}},
		{"statistics-instructor", []string{"calculator-set", "statistics-set", "visualization-set", "memory-set", "document-set", "knowledge-base-set"}},
		{"unknown-agent", []string{"calculator-set", "memory-set", "document-set", "knowledge-base-set"}}, // fallback
	}

	for _, tc := range testCases {
		enabledToolSets, err := service.GetEnabledToolSets(ctx, userID, tc.agentID)
		if err != nil {
			t.Errorf("Failed to get enabled tool sets for %s: %v", tc.agentID, err)
			continue
		}

		if len(enabledToolSets) != len(tc.expected) {
			t.Errorf("Agent %s: expected %d tool sets, got %d: %v",
				tc.agentID, len(tc.expected), len(enabledToolSets), enabledToolSets)
			continue
		}

		// Convert to map for easier checking
		enabledMap := make(map[string]bool)
		for _, toolSet := range enabledToolSets {
			enabledMap[toolSet] = true
		}

		for _, expectedToolSet := range tc.expected {
			if !enabledMap[expectedToolSet] {
				t.Errorf("Agent %s: missing expected tool set %s", tc.agentID, expectedToolSet)
			}
		}
	}
}