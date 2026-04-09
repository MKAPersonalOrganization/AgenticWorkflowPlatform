package toolreqs_test

import (
	"aicompass/foundation/logger"
	"aicompass/modules/toolreqs"
	"aicompass/modules/toolreqs/mocks"
	"aicompass/platform/toolregistry"
	"context"
	"os"
	"testing"

	toolregistry_mocks "aicompass/platform/toolregistry/mocks"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func testRedisClientToolset(t *testing.T) goredis.Cmdable {
	t.Helper()
	s := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: s.Addr()})
}

func testServiceToolset(t *testing.T) *toolreqs.Service {
	t.Helper()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)
	return toolreqs.NewService(toolreqs.ServiceConfig{
		Log:  log,
		Repo: mocks.NewInMemoryRepository(),
		RDB:  testRedisClientToolset(t),
	})
}

func TestToolSetRequirementConfigs(t *testing.T) {
	ctx := context.Background()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	toolRegistryRepo := toolregistry_mocks.NewInMemoryRepository()

	service := testServiceToolset(t)
	toolRegistry := toolregistry.NewRegistry(log, toolRegistryRepo)

	if err := toolRegistry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize tool registry: %v", err)
	}

	userID := "test-user"

	agentTypes := []string{"math-expert", "statistics-expert"}

	for _, agentType := range agentTypes {
		t.Logf("Testing no defaults for %s", agentType)

		enabledToolSets, err := service.GetEnabledToolSets(ctx, userID, agentType)
		if err != nil {
			t.Fatalf("Failed to get enabled tool sets for %s: %v", agentType, err)
		}

		if len(enabledToolSets) != 0 {
			t.Errorf("Expected %s to have no default tool sets, but got %d: %v", agentType, len(enabledToolSets), enabledToolSets)
		}
	}

	t.Log("Testing explicit tool set requirement config changes...")

	err := service.SetToolSetRequirementConfig(ctx, userID, "math-expert", "calculator-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to allow calculator-set for math-expert: %v", err)
	}

	err = service.SetToolSetRequirementConfig(ctx, userID, "math-expert", "cognitive-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to allow cognitive-set for math-expert: %v", err)
	}

	mathToolSets, err := service.GetEnabledToolSets(ctx, userID, "math-expert")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for math-expert: %v", err)
	}

	if len(mathToolSets) != 2 {
		t.Errorf("Expected math-expert to have 2 tool sets, got %d: %v", len(mathToolSets), mathToolSets)
	}

	// Block calculator-set
	err = service.SetToolSetRequirementConfig(ctx, userID, "math-expert", "calculator-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	})
	if err != nil {
		t.Fatalf("Failed to block calculator-set for math-expert: %v", err)
	}

	mathToolSets, err = service.GetEnabledToolSets(ctx, userID, "math-expert")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for math-expert: %v", err)
	}

	for _, toolSetID := range mathToolSets {
		if toolSetID == "calculator-set" {
			t.Errorf("Expected calculator-set to be blocked for math-expert, but it's still allowed")
		}
	}

	if len(mathToolSets) != 1 {
		t.Errorf("Expected math-expert to have 1 tool set after blocking, got %d: %v", len(mathToolSets), mathToolSets)
	}

	t.Logf("After blocking calculator-set, math-expert has %d tool sets: %v", len(mathToolSets), mathToolSets)

	physicsToolSets, err := service.GetEnabledToolSets(ctx, userID, "physics-expert")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for physics-expert: %v", err)
	}

	if len(physicsToolSets) != 0 {
		t.Errorf("Expected physics-expert to have no tool sets (nothing configured), got %d: %v", len(physicsToolSets), physicsToolSets)
	}

	// Allow a new tool set
	err = service.SetToolSetRequirementConfig(ctx, userID, "math-expert", "chemistry-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatalf("Failed to allow chemistry-set for math-expert: %v", err)
	}

	mathToolSetsAfterEnable, err := service.GetEnabledToolSets(ctx, userID, "math-expert")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets after allowing chemistry-set: %v", err)
	}

	chemistrySetAllowed := false
	for _, toolSetID := range mathToolSetsAfterEnable {
		if toolSetID == "chemistry-set" {
			chemistrySetAllowed = true
			break
		}
	}

	if !chemistrySetAllowed {
		t.Errorf("Expected chemistry-set to be allowed for math-expert after manual allow")
	}

	t.Logf("After allowing chemistry-set, math-expert has %d tool sets: %v", len(mathToolSetsAfterEnable), mathToolSetsAfterEnable)

	// Verify blocked tool set via HasToolSetPermission
	if service.HasToolSetPermission(ctx, userID, "math-expert", "calculator-set") {
		t.Errorf("Expected math-expert to NOT have calculator-set permission")
	}

	// Verify unconfigured tool set — fail-open (no config = allowed)
	if !service.HasToolSetPermission(ctx, userID, "physics-expert", "calculator-set") {
		t.Errorf("Expected unconfigured tool set to be allowed (fail-open)")
	}

	if !service.HasToolSetPermission(ctx, userID, "math-expert", "chemistry-set") {
		t.Errorf("Expected math-expert to have chemistry-set permission after manual allow")
	}

	t.Log("Tool set requirement config tests passed!")
}
