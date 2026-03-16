package toolreqs_test

import (
	"context"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/modules/toolreqs"
	"school-with-ai/modules/toolreqs/mocks"
	"school-with-ai/platform/toolregistry"

	toolregistry_mocks "school-with-ai/platform/toolregistry/mocks"

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

func TestToolSetPermissions(t *testing.T) {
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

	t.Log("Testing explicit tool set permission changes...")

	err := service.SetToolSetPermission(ctx, userID, "math-expert", "calculator-set", true)
	if err != nil {
		t.Fatalf("Failed to enable calculator-set for math-expert: %v", err)
	}

	err = service.SetToolSetPermission(ctx, userID, "math-expert", "cognitive-set", true)
	if err != nil {
		t.Fatalf("Failed to enable cognitive-set for math-expert: %v", err)
	}

	mathToolSets, err := service.GetEnabledToolSets(ctx, userID, "math-expert")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for math-expert: %v", err)
	}

	if len(mathToolSets) != 2 {
		t.Errorf("Expected math-expert to have 2 tool sets, got %d: %v", len(mathToolSets), mathToolSets)
	}

	err = service.SetToolSetPermission(ctx, userID, "math-expert", "calculator-set", false)
	if err != nil {
		t.Fatalf("Failed to disable calculator-set for math-expert: %v", err)
	}

	mathToolSets, err = service.GetEnabledToolSets(ctx, userID, "math-expert")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for math-expert: %v", err)
	}

	for _, toolSetID := range mathToolSets {
		if toolSetID == "calculator-set" {
			t.Errorf("Expected calculator-set to be disabled for math-expert, but it's still enabled")
		}
	}

	if len(mathToolSets) != 1 {
		t.Errorf("Expected math-expert to have 1 tool set after disabling, got %d: %v", len(mathToolSets), mathToolSets)
	}

	t.Logf("After disabling calculator-set, math-expert has %d tool sets: %v", len(mathToolSets), mathToolSets)

	physicsToolSets, err := service.GetEnabledToolSets(ctx, userID, "physics-expert")
	if err != nil {
		t.Fatalf("Failed to get enabled tool sets for physics-expert: %v", err)
	}

	if len(physicsToolSets) != 0 {
		t.Errorf("Expected physics-expert to have no tool sets (nothing configured), got %d: %v", len(physicsToolSets), physicsToolSets)
	}

	err = service.SetToolSetPermission(ctx, userID, "math-expert", "chemistry-set", true)
	if err != nil {
		t.Fatalf("Failed to enable chemistry-set for math-expert: %v", err)
	}

	mathToolSetsAfterEnable, err := service.GetEnabledToolSets(ctx, userID, "math-expert")
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
		t.Errorf("Expected chemistry-set to be enabled for math-expert after manual enable")
	}

	t.Logf("After enabling chemistry-set, math-expert has %d tool sets: %v", len(mathToolSetsAfterEnable), mathToolSetsAfterEnable)

	calcPerm, err := service.GetToolSetPermission(ctx, userID, "math-expert", "calculator-set")
	if err != nil {
		t.Fatalf("Failed to get calculator-set permission: %v", err)
	}

	if calcPerm.Enabled {
		t.Errorf("Expected calculator-set permission to be disabled, but it's enabled")
	}

	if service.HasToolSetPermission(ctx, userID, "math-expert", "calculator-set") {
		t.Errorf("Expected math-expert to NOT have calculator-set permission")
	}

	if service.HasToolSetPermission(ctx, userID, "physics-expert", "calculator-set") {
		t.Errorf("Expected physics-expert to NOT have calculator-set permission (nothing configured)")
	}

	if !service.HasToolSetPermission(ctx, userID, "math-expert", "chemistry-set") {
		t.Errorf("Expected math-expert to have chemistry-set permission after manual enable")
	}

	t.Log("Tool set permissions tests passed!")
}
