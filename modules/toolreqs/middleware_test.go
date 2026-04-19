package toolreqs_test

import (
	"aicompass/modules/toolreqs"
	"context"
	"testing"
)

// =============================================================================
// CheckConfirmation tests
// =============================================================================

func TestCheckConfirmation_NoPolicy_NoConfirmation(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	needs, hint := toolreqs.CheckConfirmation(context.Background(), nil, nil, cr,"user1", "agent1", "", "calculator", nil)
	if needs {
		t.Error("expected no confirmation when no policy exists")
	}
	if hint != "" {
		t.Errorf("expected empty hint, got %q", hint)
	}
}

func TestCheckConfirmation_GlobalPolicy_RequiresConfirmation(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("dangerous_tool", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		Hint:          "Confirm dangerous action?",
	})

	needs, hint := toolreqs.CheckConfirmation(context.Background(), nil, nil, cr,"user1", "agent1", "", "dangerous_tool", nil)
	if !needs {
		t.Error("expected confirmation when global policy exists")
	}
	if hint != "Confirm dangerous action?" {
		t.Errorf("expected hint from policy, got %q", hint)
	}
}

func TestCheckConfirmation_GlobalPolicy_NoHint_UsesDefault(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("dangerous_tool", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		// No Hint provided
	})

	needs, hint := toolreqs.CheckConfirmation(context.Background(), nil, nil, cr,"user1", "agent1", "", "dangerous_tool", nil)
	if !needs {
		t.Error("expected confirmation when global policy exists")
	}
	// getHint returns a default hint when policy has no hint.
	if hint == "" {
		t.Error("expected non-empty hint from policy's default")
	}
}

func TestCheckConfirmation_ConfirmFunc_Dynamic(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("delete_tool", &toolreqs.ConfirmationPolicy{
		ConfirmFunc: func(toolName string, args map[string]any) bool {
			hard, _ := args["hard_delete"].(bool)
			return hard
		},
		Hint: "Hard delete requires confirmation",
	})

	// Should not require confirmation for soft delete
	needs, _ := toolreqs.CheckConfirmation(context.Background(), nil, nil, cr,"user1", "agent1", "", "delete_tool", map[string]any{"hard_delete": false})
	if needs {
		t.Error("should not require confirmation for soft delete")
	}

	// Should require confirmation for hard delete
	needs, hint := toolreqs.CheckConfirmation(context.Background(), nil, nil, cr,"user1", "agent1", "", "delete_tool", map[string]any{"hard_delete": true})
	if !needs {
		t.Error("should require confirmation for hard delete")
	}
	if hint != "Hard delete requires confirmation" {
		t.Errorf("expected hint from policy, got %q", hint)
	}
}

func TestCheckConfirmation_AgentSpecificPolicy_OverridesDefault(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetDefaultPolicy("tool_x", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: false,
	})
	cr.SetAgentPolicy("agent1", "tool_x", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		Hint:          "Agent-specific confirmation",
	})

	// Agent1 should require confirmation
	needs, hint := toolreqs.CheckConfirmation(context.Background(), nil, nil, cr,"user1", "agent1", "", "tool_x", nil)
	if !needs {
		t.Error("expected confirmation for agent-specific policy")
	}
	if hint != "Agent-specific confirmation" {
		t.Errorf("expected agent-specific hint, got %q", hint)
	}

	// Agent2 should NOT require confirmation (falls back to default)
	needs, _ = toolreqs.CheckConfirmation(context.Background(), nil, nil, cr,"user1", "agent2", "", "tool_x", nil)
	if needs {
		t.Error("should not require confirmation for agent without policy")
	}
}

// =============================================================================
// BlockMiddleware tests (unchanged — still uses middleware pattern)
// =============================================================================

// Note: BlockMiddleware tests are in a separate file or integrated into
// the service tests. The middleware pattern is retained for blocking since
// it's a simple pass-through check, not a pause/resume flow.
