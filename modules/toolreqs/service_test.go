package toolreqs_test

import (
	"aicompass/foundation/logger"
	"aicompass/modules/toolreqs"
	"aicompass/modules/toolreqs/mocks"
	"context"
	"io"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func testLogger() *logger.Logger {
	return logger.New(io.Discard, logger.LevelInfo, "test", nil)
}

func testRDB(t *testing.T) goredis.Cmdable {
	t.Helper()
	s := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: s.Addr()})
}

func newTestPair(t *testing.T) (*toolreqs.Service, *toolreqs.PermissionStore) {
	t.Helper()
	log := testLogger()
	store := toolreqs.NewPermissionStore(log, mocks.NewInMemoryRepository(), testRDB(t))
	svc := toolreqs.NewService(toolreqs.ServiceConfig{Log: log, Store: store})
	return svc, store
}

// =============================================================================
// Service tests
// =============================================================================

func TestService_GetEnabledToolSets_Empty(t *testing.T) {
	t.Parallel()

	svc, _ := newTestPair(t)
	sets, err := svc.GetEnabledToolSets(context.Background(), "user1", "agent1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sets) != 0 {
		t.Errorf("expected no tool sets, got %v", sets)
	}
}

func TestService_GetEnabledToolSets_WithData(t *testing.T) {
	t.Parallel()

	svc, store := newTestPair(t)
	ctx := context.Background()

	// Set up some tool sets via requirement config.
	if err := store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "calculator", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "web-search", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	}); err != nil {
		t.Fatal(err)
	}

	sets, err := svc.GetEnabledToolSets(ctx, "user1", "agent1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sets) != 2 {
		t.Fatalf("expected 2 tool sets, got %d", len(sets))
	}
}

func TestService_HasPermission_Allowed(t *testing.T) {
	t.Parallel()

	svc, store := newTestPair(t)
	ctx := context.Background()

	// Allow tool set and individual tool via requirement config.
	store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "calc-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	store.SetToolRequirementConfig(ctx, "user1", "agent1", "calc-set", "add", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})

	if !svc.HasPermission(ctx, "user1", "agent1", "calc-set", "add") {
		t.Error("expected tool to be allowed")
	}
}

func TestService_HasPermission_Blocked(t *testing.T) {
	t.Parallel()

	svc, store := newTestPair(t)
	ctx := context.Background()

	// Allow tool set but block individual tool via requirement config.
	store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "calc-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	store.SetToolRequirementConfig(ctx, "user1", "agent1", "calc-set", "add", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	})

	if svc.HasPermission(ctx, "user1", "agent1", "calc-set", "add") {
		t.Error("expected tool to be blocked")
	}
}

func TestService_HasToolSetPermission(t *testing.T) {
	t.Parallel()

	svc, store := newTestPair(t)
	ctx := context.Background()

	// Not configured → denied (fail-closed: no config means no permission).
	if svc.HasToolSetPermission(ctx, "user1", "agent1", "unconfigured-set") {
		t.Error("expected unconfigured tool set to be denied (fail-closed)")
	}

	// Explicitly allowed → allowed.
	store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "allowed-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if !svc.HasToolSetPermission(ctx, "user1", "agent1", "allowed-set") {
		t.Error("expected explicitly allowed tool set to be allowed")
	}

	// Explicitly blocked → blocked.
	store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "blocked-set", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionBlock,
	})
	if svc.HasToolSetPermission(ctx, "user1", "agent1", "blocked-set") {
		t.Error("expected explicitly blocked tool set to be blocked")
	}
}

func TestService_CheckConfirmation_Default(t *testing.T) {
	t.Parallel()

	svc, _ := newTestPair(t)
	needs, _ := svc.CheckConfirmation(context.Background(), "user1", "agent1", "", "calculator", nil)
	if needs {
		t.Error("calculator should not require confirmation by default")
	}
}
