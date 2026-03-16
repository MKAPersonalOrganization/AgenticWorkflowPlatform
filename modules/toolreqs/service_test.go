package toolreqs_test

import (
	"context"
	"io"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/modules/toolreqs"
	"school-with-ai/modules/toolreqs/mocks"

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

func newTestService(t *testing.T) *toolreqs.Service {
	t.Helper()
	return toolreqs.NewService(toolreqs.ServiceConfig{
		Log:  testLogger(),
		Repo: mocks.NewInMemoryRepository(),
		RDB:  testRDB(t),
	})
}

// =============================================================================
// Service tests
// =============================================================================

func TestService_GetEnabledToolSets_Empty(t *testing.T) {
	svc := newTestService(t)
	sets, err := svc.GetEnabledToolSets(context.Background(), "user1", "agent1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sets) != 0 {
		t.Errorf("expected no tool sets, got %v", sets)
	}
}

func TestService_GetEnabledToolSets_WithData(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Set up some tool sets.
	if err := svc.SetToolSetPermission(ctx, "user1", "agent1", "calculator", true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetToolSetPermission(ctx, "user1", "agent1", "web-search", true); err != nil {
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
	svc := newTestService(t)
	ctx := context.Background()

	// Enable tool set and individual tool.
	svc.SetToolSetPermission(ctx, "user1", "agent1", "calc-set", true)
	svc.SetPermission(ctx, "user1", "agent1", "calc-set", "add", true)

	if !svc.HasPermission(ctx, "user1", "agent1", "calc-set", "add") {
		t.Error("expected tool to be allowed")
	}
}

func TestService_HasPermission_Blocked(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Enable tool set but disable individual tool.
	svc.SetToolSetPermission(ctx, "user1", "agent1", "calc-set", true)
	svc.SetPermission(ctx, "user1", "agent1", "calc-set", "add", false)

	if svc.HasPermission(ctx, "user1", "agent1", "calc-set", "add") {
		t.Error("expected tool to be blocked")
	}
}

func TestService_HasToolSetPermission(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Not configured → blocked.
	if svc.HasToolSetPermission(ctx, "user1", "agent1", "disabled-set") {
		t.Error("expected tool set to be blocked")
	}
}

func TestService_EvaluateTool_WithConfirmation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Enable permissions so we can test confirmation.
	svc.SetToolSetPermission(ctx, "user1", "agent1", "tools", true)
	svc.SetPermission(ctx, "user1", "agent1", "tools", "dangerous_tool", true)

	// Configure confirmation for the tool.
	svc.Confirmation().SetGlobalPolicy("dangerous_tool", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		Hint:          "This is dangerous!",
	})

	decision, err := svc.EvaluateTool(ctx, &toolreqs.ToolEvalRequest{
		UserID:    "user1",
		AgentID:   "agent1",
		ToolSetID: "tools",
		ToolID:    "dangerous_tool",
		ToolName:  "dangerous_tool",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Error("tool should be allowed (confirmation doesn't block)")
	}
	if !decision.NeedsConfirm {
		t.Error("tool should need confirmation")
	}
	if decision.Confirmation == nil {
		t.Fatal("expected confirmation request")
	}
	if decision.Confirmation.Hint != "This is dangerous!" {
		t.Errorf("expected hint 'This is dangerous!', got %q", decision.Confirmation.Hint)
	}
}

func TestService_EvaluateTool_PermissionBlockOverridesConfirm(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	// Tool set disabled.
	svc.SetToolSetPermission(ctx, "user1", "agent1", "set1", false)

	svc.Confirmation().SetGlobalPolicy("tool_x", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
	})

	decision, err := svc.EvaluateTool(ctx, &toolreqs.ToolEvalRequest{
		UserID:    "user1",
		AgentID:   "agent1",
		ToolSetID: "set1",
		ToolID:    "tool_x",
		ToolName:  "tool_x",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Error("tool should be blocked by permission")
	}
	if decision.NeedsConfirm {
		t.Error("block should override confirmation")
	}
}

func TestService_RegisterRequirement(t *testing.T) {
	svc := newTestService(t)

	// Register a custom requirement that blocks everything.
	svc.RegisterRequirement(&staticRequirement{
		kind:   "custom",
		action: toolreqs.ActionBlock,
		reason: "custom block",
	})

	decision, err := svc.EvaluateTool(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "any_tool",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Error("custom requirement should block the tool")
	}
}

func TestService_RequiresConfirmation(t *testing.T) {
	svc := newTestService(t)
	svc.Confirmation().SetDefaultPolicy("web_search", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
	})

	if !svc.RequiresConfirmation(context.Background(), "agent1", "web_search", nil) {
		t.Error("expected confirmation to be required")
	}
	if svc.RequiresConfirmation(context.Background(), "agent1", "calculator", nil) {
		t.Error("calculator should not require confirmation")
	}
}

func TestService_ConfirmationMiddleware_NotNil(t *testing.T) {
	svc := newTestService(t)
	mw := svc.ConfirmationMiddleware("agent1")
	if mw == nil {
		t.Error("expected non-nil middleware")
	}
}

// =============================================================================
// Test helpers — staticRequirement is shared from evaluator_test.go
// =============================================================================
