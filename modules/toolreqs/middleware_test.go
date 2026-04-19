package toolreqs_test

import (
	"aicompass/modules/toolreqs"
	"context"
	"testing"
)

func TestCheckConfirmation_NilStore_NoConfirmation(t *testing.T) {
	needs, hint := toolreqs.CheckConfirmation(context.Background(), nil, nil, "user1", "agent1", "", "calculator", nil)
	if needs {
		t.Error("expected no confirmation when store is nil")
	}
	if hint != "" {
		t.Errorf("expected empty hint, got %q", hint)
	}
}

func TestCheckConfirmation_EmptyUser_NoConfirmation(t *testing.T) {
	_, store := newTestPair(t)
	needs, _ := toolreqs.CheckConfirmation(context.Background(), store, nil, "", "agent1", "set1", "tool_x", nil)
	if needs {
		t.Error("expected no confirmation when userID is empty")
	}
}

func TestCheckConfirmation_ToolLevelConfig_RequiresConfirmation(t *testing.T) {
	_, store := newTestPair(t)
	ctx := context.Background()

	err := store.SetToolRequirementConfig(ctx, "user1", "agent1", "set1", "file_delete", toolreqs.ToolRequirementConfig{
		Action:           toolreqs.RequirementActionRequireConfirmation,
		ConfirmationHint: "Really delete?",
	})
	if err != nil {
		t.Fatal(err)
	}

	needs, hint := toolreqs.CheckConfirmation(ctx, store, nil, "user1", "agent1", "set1", "file_delete", nil)
	if !needs {
		t.Error("expected tool-level require_confirmation to trigger")
	}
	if hint != "Really delete?" {
		t.Errorf("expected hint from tool config, got %q", hint)
	}
}

func TestCheckConfirmation_ToolSetLevelConfig_RequiresConfirmation(t *testing.T) {
	_, store := newTestPair(t)
	ctx := context.Background()

	err := store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "set1", toolreqs.ToolRequirementConfig{
		Action:           toolreqs.RequirementActionRequireConfirmation,
		ConfirmationHint: "Tool-set gate",
	})
	if err != nil {
		t.Fatal(err)
	}

	needs, hint := toolreqs.CheckConfirmation(ctx, store, nil, "user1", "agent1", "set1", "any_tool", nil)
	if !needs {
		t.Error("expected tool-set require_confirmation to trigger")
	}
	if hint != "Tool-set gate" {
		t.Errorf("expected hint from tool-set config, got %q", hint)
	}
}

func TestCheckConfirmation_AllowedToolSet_NoConfirmation(t *testing.T) {
	_, store := newTestPair(t)
	ctx := context.Background()

	err := store.SetToolSetRequirementConfig(ctx, "user1", "agent1", "set1", toolreqs.ToolRequirementConfig{
		Action: toolreqs.RequirementActionAllow,
	})
	if err != nil {
		t.Fatal(err)
	}

	needs, _ := toolreqs.CheckConfirmation(ctx, store, nil, "user1", "agent1", "set1", "any_tool", nil)
	if needs {
		t.Error("expected no confirmation for an allow-action tool set")
	}
}
