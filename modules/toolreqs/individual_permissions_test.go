package toolreqs_test

import (
	"aicompass/foundation/logger"
	"aicompass/modules/toolreqs"
	"aicompass/modules/toolreqs/mocks"
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func testRedisClientIndiv(t *testing.T) goredis.Cmdable {
	t.Helper()
	s := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: s.Addr()})
}

func testPairIndiv(t *testing.T) (*toolreqs.Service, *toolreqs.PermissionStore) {
	t.Helper()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	store := toolreqs.NewPermissionStore(log, mocks.NewInMemoryRepository(), testRedisClientIndiv(t))
	svc := toolreqs.NewService(toolreqs.ServiceConfig{Log: log, Store: store})
	return svc, store
}

func TestIndividualToolRequirementConfig(t *testing.T) {
	t.Parallel()

	service, store := testPairIndiv(t)
	ctx := context.Background()

	agentID := "math-instructor"
	toolSetID := "calculator-set"
	toolID := "add"
	userID := "test-user"

	// Test: Set individual tool requirement config to "allow"
	err := store.SetToolRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolID(toolID), toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to set tool requirement config: %v", err)
	}

	// Test: Tool should be allowed
	hasPermission := service.HasPermission(ctx, userID, agentID, toolSetID, toolID)
	if !hasPermission {
		t.Fatal("Expected tool to be allowed")
	}

	// Test: Get requirement config
	cfg, err := store.GetToolRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolID(toolID))
	if err != nil {
		t.Fatalf("Failed to get tool requirement config: %v", err)
	}

	if cfg.Action != toolreqs.RequirementActionAllow {
		t.Errorf("Expected action %s, got %s", toolreqs.RequirementActionAllow, cfg.Action)
	}

	// Test: Block the tool
	err = store.SetToolRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolID(toolID), toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	})
	if err != nil {
		t.Fatalf("Failed to block tool: %v", err)
	}

	hasPermission = service.HasPermission(ctx, userID, agentID, toolSetID, toolID)
	if hasPermission {
		t.Fatal("Blocked tool should not have permission")
	}
}

func TestToolSetRequirementInheritance(t *testing.T) {
	t.Parallel()

	service, store := testPairIndiv(t)
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-instructor"
	toolSetID := "calculator-set"
	toolID := "multiply"

	// Test: Tool inherits permission from tool set when no explicit config exists

	// Allow the tool set
	err := store.SetToolSetRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to allow tool set: %v", err)
	}

	// Tool should inherit allowed status from tool set
	hasPermission := service.HasPermission(ctx, userID, agentID, toolSetID, toolID)
	if !hasPermission {
		t.Fatal("Tool should inherit permission from allowed tool set")
	}

	// Block the tool set
	err = store.SetToolSetRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	})
	if err != nil {
		t.Fatalf("Failed to block tool set: %v", err)
	}

	// Tool should inherit blocked status from tool set
	hasPermission = service.HasPermission(ctx, userID, agentID, toolSetID, toolID)
	if hasPermission {
		t.Fatal("Tool should inherit blocked status from tool set")
	}
}

func TestToolRequirementOverridesToolSet(t *testing.T) {
	t.Parallel()

	service, store := testPairIndiv(t)
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-instructor"
	toolSetID := "calculator-set"
	toolID := "divide"

	// Block the tool set
	err := store.SetToolSetRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	})
	if err != nil {
		t.Fatalf("Failed to block tool set: %v", err)
	}

	// Explicitly allow the individual tool
	err = store.SetToolRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolID(toolID), toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to allow individual tool: %v", err)
	}

	// Individual tool should be allowed despite tool set being blocked
	hasPermission := service.HasPermission(ctx, userID, agentID, toolSetID, toolID)
	if !hasPermission {
		t.Fatal("Individual tool config should override tool set config")
	}

	// Tool set should still be blocked
	hasToolSetPermission := service.HasToolSetPermission(ctx, userID, agentID, toolSetID)
	if hasToolSetPermission {
		t.Fatal("Tool set should remain blocked")
	}
}

func TestRequireConfirmationConfig(t *testing.T) {
	t.Parallel()

	service, store := testPairIndiv(t)
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-instructor"
	toolSetID := "calculator-set"
	toolID := "divide"

	// Set require_confirmation on the tool
	err := store.SetToolRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolID(toolID), toolreqs.ToolRequirementConfig{
		Action:              toolreqs.RequirementActionRequireConfirmation,
		RequireConfirmation: true,
		ConfirmationHint:    "Division can produce unexpected results",
	})
	if err != nil {
		t.Fatalf("Failed to set require_confirmation config: %v", err)
	}

	// Tool should still be "allowed" (not blocked)
	hasPermission := service.HasPermission(ctx, userID, agentID, toolSetID, toolID)
	if !hasPermission {
		t.Fatal("require_confirmation tool should still have permission (not blocked)")
	}

	// Config should have confirmation details
	cfg, err := store.GetToolRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolID(toolID))
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if cfg.Action != toolreqs.RequirementActionRequireConfirmation {
		t.Errorf("Expected action %s, got %s", toolreqs.RequirementActionRequireConfirmation, cfg.Action)
	}

	if !cfg.RequireConfirmation {
		t.Error("RequireConfirmation should be true")
	}

	if cfg.ConfirmationHint != "Division can produce unexpected results" {
		t.Errorf("Unexpected confirmation hint: %s", cfg.ConfirmationHint)
	}
}

func TestListToolRequirementConfigs(t *testing.T) {
	t.Parallel()

	_, store := testPairIndiv(t)
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-instructor"
	toolSetID := "calculator-set"

	// Set configs for multiple tools
	tools := map[string]toolreqs.RequirementAction{
		"add":      toolreqs.RequirementActionAllow,
		"subtract": toolreqs.RequirementActionBlock,
		"multiply": toolreqs.RequirementActionRequireConfirmation,
	}

	for toolID, action := range tools {
		err := store.SetToolRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID), toolreqs.ToolID(toolID), toolreqs.ToolRequirementConfig{
			Action: action,
		})
		if err != nil {
			t.Fatalf("Failed to set config for %s: %v", toolID, err)
		}
	}

	// List all configs in the tool set
	configs, err := store.ListToolRequirementConfigs(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), toolreqs.ToolSetID(toolSetID))
	if err != nil {
		t.Fatalf("Failed to list configs: %v", err)
	}

	if len(configs) != 3 {
		t.Errorf("Expected 3 configs, got %d", len(configs))
	}

	for toolID, expectedAction := range tools {
		cfg, ok := configs[toolID]
		if !ok {
			t.Errorf("Missing config for tool %s", toolID)
			continue
		}
		if cfg.Action != expectedAction {
			t.Errorf("Tool %s: expected action %s, got %s", toolID, expectedAction, cfg.Action)
		}
	}
}

func TestGetAllowedToolSets(t *testing.T) {
	t.Parallel()

	service, store := testPairIndiv(t)
	ctx := context.Background()

	userID := "test-user"
	agentID := "math-instructor"

	// Allow two tool sets, block one
	err := store.SetToolSetRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), "calc-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to allow calc-set: %v", err)
	}

	err = store.SetToolSetRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), "graph-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to allow graph-set: %v", err)
	}

	err = store.SetToolSetRequirementConfig(ctx, toolreqs.UserID(userID), toolreqs.AgentID(agentID), "blocked-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	})
	if err != nil {
		t.Fatalf("Failed to block blocked-set: %v", err)
	}

	// GetEnabledToolSets should return only allowed ones
	allowed, err := service.GetEnabledToolSets(ctx, userID, agentID)
	if err != nil {
		t.Fatalf("Failed to get allowed tool sets: %v", err)
	}

	if len(allowed) != 2 {
		t.Errorf("Expected 2 allowed tool sets, got %d: %v", len(allowed), allowed)
	}

	expectedSets := map[string]bool{"calc-set": true, "graph-set": true}
	for _, id := range allowed {
		if !expectedSets[id] {
			t.Errorf("Unexpected allowed tool set: %s", id)
		}
		delete(expectedSets, id)
	}

	if len(expectedSets) > 0 {
		t.Errorf("Missing expected allowed tool sets: %v", expectedSets)
	}
}
