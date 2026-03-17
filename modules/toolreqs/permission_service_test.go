package toolreqs_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/modules/toolreqs"
	"school-with-ai/modules/toolreqs/mocks"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func testRedisClient(t *testing.T) goredis.Cmdable {
	t.Helper()
	s := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: s.Addr()})
}

func testService(t *testing.T) *toolreqs.Service {
	t.Helper()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	return toolreqs.NewService(toolreqs.ServiceConfig{
		Log:  log,
		Repo: mocks.NewInMemoryRepository(),
		RDB:  testRedisClient(t),
	})
}

func TestToolSetRequirementConfigService(t *testing.T) {
	service := testService(t)
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-expert"
	toolSetID := "calculator-set"

	// Allow a tool set via requirement config
	if err := service.SetToolSetRequirementConfig(ctx, userID, agentID, toolSetID, toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	}); err != nil {
		t.Fatalf("Failed to allow tool set: %v", err)
	}

	// Tool set should be allowed
	hasPermission := service.HasToolSetPermission(ctx, userID, agentID, toolSetID)
	if !hasPermission {
		t.Fatal("Expected agent to have permission for allowed tool set")
	}

	// Should appear in enabled (allowed) tool sets
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
		t.Fatal("Tool set should be in allowed tool sets list")
	}
}

func TestToolSetBlockingViaRequirementConfig(t *testing.T) {
	service := testService(t)
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-expert"
	toolSetID := "calculator-set"

	// First allow the tool set
	if err := service.SetToolSetRequirementConfig(ctx, userID, agentID, toolSetID, toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	}); err != nil {
		t.Fatalf("Failed to allow tool set: %v", err)
	}

	// Then block it
	if err := service.SetToolSetRequirementConfig(ctx, userID, agentID, toolSetID, toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	}); err != nil {
		t.Fatalf("Failed to block tool set: %v", err)
	}

	// Verify it's blocked
	hasPermission := service.HasToolSetPermission(ctx, userID, agentID, toolSetID)
	if hasPermission {
		t.Fatal("Expected agent to NOT have permission for blocked tool set")
	}

	// Verify blocked tool set is not in allowed list
	enabledToolSets, err := service.GetEnabledToolSets(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets: %v", err)
	}

	for _, id := range enabledToolSets {
		if id == toolSetID {
			t.Fatal("Blocked tool set should not be in allowed tool sets list")
		}
	}
}

func TestDefaultToolSets(t *testing.T) {
	service := testService(t)
	ctx := context.Background()

	userID := "test-user"

	// With the decoupled architecture, agents start with no default tool sets.
	// All tool sets must be explicitly allowed per agent.
	testCases := []struct {
		agentID  string
		expected int
	}{
		{"math-expert", 0},
		{"statistics-expert", 0},
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
