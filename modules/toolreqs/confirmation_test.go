package toolreqs_test

import (
	"context"
	"testing"

	"school-with-ai/modules/toolreqs"
)

// =============================================================================
// ConfirmationRequirement tests
// =============================================================================

func TestConfirmation_NoPolicy_NoConfirmation(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	if cr.RequiresConfirmation("agent1", "calculator", nil) {
		t.Error("expected no confirmation when no policy is set")
	}
}

func TestConfirmation_GlobalPolicy_AlwaysConfirm(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("file_delete", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		Hint:          "This will permanently delete files. Are you sure?",
	})

	if !cr.RequiresConfirmation("any-agent", "file_delete", nil) {
		t.Error("global policy should require confirmation for any agent")
	}
	if cr.RequiresConfirmation("any-agent", "calculator", nil) {
		t.Error("other tools should not require confirmation")
	}
}

func TestConfirmation_GlobalPolicy_OverridesAgentPolicy(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()

	// Global says always confirm.
	cr.SetGlobalPolicy("file_delete", &toolreqs.ConfirmationPolicy{AlwaysConfirm: true})

	// Agent-specific policy is irrelevant — global wins.
	// (No agent policy set, but global should still trigger.)
	if !cr.RequiresConfirmation("agent1", "file_delete", nil) {
		t.Error("global policy should always require confirmation")
	}
}

func TestConfirmation_AgentPolicy(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetAgentPolicy("agent1", "web_search", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		Hint:          "Allow web search?",
	})

	if !cr.RequiresConfirmation("agent1", "web_search", nil) {
		t.Error("agent1 should require confirmation for web_search")
	}
	if cr.RequiresConfirmation("agent2", "web_search", nil) {
		t.Error("agent2 should NOT require confirmation (no policy)")
	}
}

func TestConfirmation_DefaultPolicy(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetDefaultPolicy("code_execute", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
	})

	// All agents should pick up the default.
	if !cr.RequiresConfirmation("agent1", "code_execute", nil) {
		t.Error("default policy should apply to agent1")
	}
	if !cr.RequiresConfirmation("agent2", "code_execute", nil) {
		t.Error("default policy should apply to agent2")
	}
}

func TestConfirmation_AgentOverridesDefault(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()

	// Default says confirm.
	cr.SetDefaultPolicy("web_search", &toolreqs.ConfirmationPolicy{AlwaysConfirm: true})

	// Agent-specific says no confirm (ConfirmFunc returns false).
	cr.SetAgentPolicy("trusted-agent", "web_search", &toolreqs.ConfirmationPolicy{
		ConfirmFunc: func(string, map[string]any) bool { return false },
	})

	if cr.RequiresConfirmation("trusted-agent", "web_search", nil) {
		t.Error("agent-specific policy should override default")
	}
	if !cr.RequiresConfirmation("other-agent", "web_search", nil) {
		t.Error("other agents should still use the default policy")
	}
}

func TestConfirmation_DynamicConfirmFunc(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetDefaultPolicy("file_write", &toolreqs.ConfirmationPolicy{
		ConfirmFunc: func(toolName string, args map[string]any) bool {
			// Only confirm when overwrite is true.
			overwrite, _ := args["overwrite"].(bool)
			return overwrite
		},
	})

	// No overwrite — no confirmation.
	if cr.RequiresConfirmation("agent1", "file_write", map[string]any{"path": "/tmp/a"}) {
		t.Error("should not require confirmation without overwrite flag")
	}

	// With overwrite — requires confirmation.
	if !cr.RequiresConfirmation("agent1", "file_write", map[string]any{"overwrite": true}) {
		t.Error("should require confirmation with overwrite flag")
	}
}

func TestConfirmation_RemovePolicy(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("file_delete", &toolreqs.ConfirmationPolicy{AlwaysConfirm: true})

	if !cr.RequiresConfirmation("agent1", "file_delete", nil) {
		t.Fatal("setup: expected confirmation to be required")
	}

	cr.RemoveGlobalPolicy("file_delete")

	if cr.RequiresConfirmation("agent1", "file_delete", nil) {
		t.Error("confirmation should no longer be required after removing policy")
	}
}

func TestConfirmation_RemoveAgentPolicy(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetAgentPolicy("agent1", "tool_x", &toolreqs.ConfirmationPolicy{AlwaysConfirm: true})

	cr.RemoveAgentPolicy("agent1", "tool_x")

	if cr.RequiresConfirmation("agent1", "tool_x", nil) {
		t.Error("confirmation should no longer be required after removing agent policy")
	}
}

func TestConfirmation_Evaluate_RequiresConfirmation(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("file_delete", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		Hint:          "Are you sure?",
	})

	result, err := cr.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		AgentID:  "agent1",
		ToolName: "file_delete",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != toolreqs.ActionConfirm {
		t.Errorf("expected ActionConfirm, got %v", result.Action)
	}
	if result.Confirmation == nil {
		t.Fatal("expected confirmation request to be populated")
	}
	if result.Confirmation.Hint != "Are you sure?" {
		t.Errorf("expected hint 'Are you sure?', got %q", result.Confirmation.Hint)
	}
}

func TestConfirmation_Evaluate_NoConfirmation(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()

	result, err := cr.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		AgentID:  "agent1",
		ToolName: "calculator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Action != toolreqs.ActionAllow {
		t.Errorf("expected ActionAllow, got %v", result.Action)
	}
	if !result.Satisfied {
		t.Error("expected requirement to be satisfied")
	}
}
