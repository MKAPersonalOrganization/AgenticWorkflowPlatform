package tool_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/modules/permissions/tool"
	"school-with-ai/modules/permissions/tool/mocks"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func testRedisClient(t *testing.T) *goredis.Client {
	t.Helper()
	s := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: s.Addr()})
}

func TestToolSetPermissionsService(t *testing.T) {
	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Create service with in-memory repository
	service := tool.NewService(log, mocks.NewInMemoryRepository(), testRedisClient(t))

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
	service := tool.NewService(log, mocks.NewInMemoryRepository(), testRedisClient(t))
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
	service := tool.NewService(log, mocks.NewInMemoryRepository(), testRedisClient(t))
	ctx := context.Background()

	userID := "test-user"

	// With the decoupled architecture, agents start with no default tool sets.
	// All tool sets must be explicitly enabled per agent.
	testCases := []struct {
		agentID  string
		expected int
	}{
		{"math-instructor", 0},
		{"statistics-instructor", 0},
		{"unknown-agent", 0},
	}

	for _, tc := range testCases {
		enabledToolSets, err := service.GetEnabledToolSets(ctx, userID, tc.agentID)
		if err != nil {
			t.Errorf("Failed to get enabled tool sets for %s: %v", tc.agentID, err)
			continue
		}

		if len(enabledToolSets) != tc.expected {
			t.Errorf("Agent %s: expected %d tool sets, got %d: %v",
				tc.agentID, tc.expected, len(enabledToolSets), enabledToolSets)
		}
	}
}
