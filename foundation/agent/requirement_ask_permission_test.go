package agent_test

import (
	"context"
	"errors"
	"testing"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agent/io"
)

// mockTool implements agent.Tool for testing.
type mockTool struct {
	name string
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return "mock tool" }
func (t *mockTool) Schema() *agent.Schema {
	return &agent.Schema{Type: agent.TypeObject}
}

// =============================================================================
// AskPermission middleware tests
// =============================================================================

func TestAskPermission_auto_approves_without_handler(t *testing.T) {
	middleware := agent.AskPermission("web_search")

	executed := false
	handler := middleware(agent.HandlerFunc(func(c *agent.Context) error {
		executed = true
		return nil
	}))

	ctx := agent.NewFunctionContext(
		context.Background(), // no IO handler set
		&mockTool{name: "web_search"},
		map[string]any{},
	)

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected tool to execute (auto-approve with no handler)")
	}
}

func TestAskPermission_approves_with_NoopHandler(t *testing.T) {
	middleware := agent.AskPermission("web_search")

	executed := false
	handler := middleware(agent.HandlerFunc(func(c *agent.Context) error {
		executed = true
		return nil
	}))

	ioCtx := io.WithHandler(context.Background(), io.NewNoopHandler())
	ctx := agent.NewFunctionContext(
		ioCtx,
		&mockTool{name: "web_search"},
		map[string]any{},
	)

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected tool to execute (NoopHandler approves)")
	}
}

func TestAskPermission_denies_with_DenyHandler(t *testing.T) {
	middleware := agent.AskPermission("web_search")

	executed := false
	handler := middleware(agent.HandlerFunc(func(c *agent.Context) error {
		executed = true
		return nil
	}))

	ioCtx := io.WithHandler(context.Background(), io.NewDenyHandler())
	ctx := agent.NewFunctionContext(
		ioCtx,
		&mockTool{name: "web_search"},
		map[string]any{},
	)

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executed {
		t.Fatal("expected tool NOT to execute (DenyHandler denies)")
	}
	// Should have a denial result
	if ctx.Result == nil {
		t.Fatal("expected denial result to be set")
	}
	result, ok := ctx.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", ctx.Result)
	}
	if denied, _ := result["denied"].(bool); !denied {
		t.Fatal("expected denied=true in result")
	}
}

func TestAskPermission_pauses_with_PauseHandler(t *testing.T) {
	middleware := agent.AskPermission("web_search")

	executed := false
	handler := middleware(agent.HandlerFunc(func(c *agent.Context) error {
		executed = true
		return nil
	}))

	ioCtx := io.WithHandler(context.Background(), io.NewPauseHandler())
	ctx := agent.NewFunctionContext(
		ioCtx,
		&mockTool{name: "web_search"},
		map[string]any{},
	)

	err := handler.Handle(ctx)
	if executed {
		t.Fatal("expected tool NOT to execute (PauseHandler pauses)")
	}

	// Should return ErrAuthRequired sentinel error
	var authErr *io.ErrAuthRequired
	if !errors.As(err, &authErr) {
		t.Fatalf("expected ErrAuthRequired, got %T: %v", err, err)
	}
	if authErr.Resource != "tool:web_search" {
		t.Errorf("expected resource 'tool:web_search', got %q", authErr.Resource)
	}
}

func TestAskPermission_skips_non_matching_tools(t *testing.T) {
	// Only require permission for "web_search"
	middleware := agent.AskPermission("web_search")

	executed := false
	handler := middleware(agent.HandlerFunc(func(c *agent.Context) error {
		executed = true
		return nil
	}))

	// Execute "calculator" — should NOT require permission
	ioCtx := io.WithHandler(context.Background(), io.NewDenyHandler())
	ctx := agent.NewFunctionContext(
		ioCtx,
		&mockTool{name: "calculator"},
		map[string]any{},
	)

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected calculator to execute (not in approval list)")
	}
}

func TestAskPermission_all_tools_when_empty(t *testing.T) {
	// Empty tool list = ALL tools require approval
	middleware := agent.AskPermission()

	executed := false
	handler := middleware(agent.HandlerFunc(func(c *agent.Context) error {
		executed = true
		return nil
	}))

	ioCtx := io.WithHandler(context.Background(), io.NewDenyHandler())
	ctx := agent.NewFunctionContext(
		ioCtx,
		&mockTool{name: "any_tool"},
		map[string]any{},
	)

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executed {
		t.Fatal("expected tool to be denied (AskPermission() with no args = all tools)")
	}
}

func TestAskPermission_passes_non_function_contexts(t *testing.T) {
	middleware := agent.AskPermission("web_search")

	executed := false
	handler := middleware(agent.HandlerFunc(func(c *agent.Context) error {
		executed = true
		return nil
	}))

	// Create a Chat context (not Function) — should pass through
	ctx := agent.NewChatContext(
		context.Background(),
		nil,
		nil,
		nil,
	)

	err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Fatal("expected pass-through for non-function context")
	}
}

// =============================================================================
// AskPermissionRequirement tests
// =============================================================================

func TestAskPermissionRequirement_implements_Requirement(t *testing.T) {
	req := agent.NewAskPermissionRequirement("web_search")

	// Run should return no rules
	rules, err := req.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Fatalf("expected no rules, got %d", len(rules))
	}
}

func TestAskPermissionRequirement_provides_middleware(t *testing.T) {
	req := agent.NewAskPermissionRequirement("web_search")

	// Should implement FunctionMiddlewareProvider
	var fmp agent.FunctionMiddlewareProvider = req
	mw := fmp.FunctionMiddleware()
	if mw == nil {
		t.Fatal("expected middleware, got nil")
	}
}

func TestAskPermissionRequirement_provides_prompt(t *testing.T) {
	req := agent.NewAskPermissionRequirement("web_search", "file_write")

	var pp agent.PromptProvider = req
	prompt := pp.PromptAddition()
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestAskPermissionRequirement_prompt_all_tools(t *testing.T) {
	req := agent.NewAskPermissionRequirement()

	var pp agent.PromptProvider = req
	prompt := pp.PromptAddition()
	if prompt != "Note: All tool executions require user approval before running." {
		t.Errorf("unexpected prompt: %q", prompt)
	}
}
