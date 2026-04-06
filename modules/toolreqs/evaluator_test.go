package toolreqs_test

import (
	"context"
	"testing"

	"aicompass/modules/toolreqs"
)

// =============================================================================
// Evaluator tests
// =============================================================================

func TestEvaluator_NoRequirements_AllowsByDefault(t *testing.T) {
	ev := toolreqs.NewEvaluator()
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "calculator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Error("expected tool to be allowed when no requirements are registered")
	}
	if decision.Blocked {
		t.Error("expected tool NOT to be blocked")
	}
}

func TestEvaluator_SingleAllow(t *testing.T) {
	ev := toolreqs.NewEvaluator(&staticRequirement{
		kind:   toolreqs.RequirementPermission,
		action: toolreqs.ActionAllow,
	})
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "calculator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Error("expected tool to be allowed")
	}
}

func TestEvaluator_SingleBlock(t *testing.T) {
	ev := toolreqs.NewEvaluator(&staticRequirement{
		kind:   toolreqs.RequirementPermission,
		action: toolreqs.ActionBlock,
		reason: "disabled by admin",
	})
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "calculator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Error("expected tool to be blocked")
	}
	if !decision.Blocked {
		t.Error("expected Blocked flag to be true")
	}
	if len(decision.Reasons) == 0 {
		t.Error("expected reasons to be populated")
	}
}

func TestEvaluator_BlockOverridesForce(t *testing.T) {
	ev := toolreqs.NewEvaluator(
		&staticRequirement{kind: toolreqs.RequirementConditional, action: toolreqs.ActionForce, priority: 10},
		&staticRequirement{kind: toolreqs.RequirementPermission, action: toolreqs.ActionBlock},
	)
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "calculator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Allowed {
		t.Error("block should override force — tool should not be allowed")
	}
	if decision.Forced {
		t.Error("block should clear forced flag")
	}
}

func TestEvaluator_BlockOverridesConfirm(t *testing.T) {
	ev := toolreqs.NewEvaluator(
		&staticRequirement{kind: toolreqs.RequirementConfirmation, action: toolreqs.ActionConfirm},
		&staticRequirement{kind: toolreqs.RequirementPermission, action: toolreqs.ActionBlock},
	)
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "calculator",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.NeedsConfirm {
		t.Error("block should clear NeedsConfirm")
	}
}

func TestEvaluator_ConfirmDoesNotBlock(t *testing.T) {
	ev := toolreqs.NewEvaluator(
		&staticRequirement{kind: toolreqs.RequirementPermission, action: toolreqs.ActionAllow},
		&staticRequirement{
			kind:   toolreqs.RequirementConfirmation,
			action: toolreqs.ActionConfirm,
			confirmation: &toolreqs.ConfirmationRequest{
				ToolName: "web_search",
				Hint:     "Allow web search?",
			},
		},
	)
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "web_search",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Error("confirm should not block — tool should be allowed")
	}
	if !decision.NeedsConfirm {
		t.Error("expected NeedsConfirm to be true")
	}
	if decision.Confirmation == nil {
		t.Error("expected Confirmation to be populated")
	}
}

func TestEvaluator_Force_HighestPriorityWins(t *testing.T) {
	ev := toolreqs.NewEvaluator(
		&staticRequirement{kind: toolreqs.RequirementConditional, action: toolreqs.ActionForce, priority: 5, reason: "low priority"},
		&staticRequirement{kind: toolreqs.RequirementConditional, action: toolreqs.ActionForce, priority: 15, reason: "high priority"},
	)
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "think",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Forced {
		t.Error("expected tool to be forced")
	}
}

func TestEvaluator_Hide(t *testing.T) {
	ev := toolreqs.NewEvaluator(
		&staticRequirement{kind: toolreqs.RequirementConditional, action: toolreqs.ActionHide},
	)
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "internal_tool",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Hidden {
		t.Error("expected tool to be hidden")
	}
}

func TestEvaluator_ErroringRequirement_NonFatal(t *testing.T) {
	ev := toolreqs.NewEvaluator(
		&errorRequirement{},
		&staticRequirement{kind: toolreqs.RequirementPermission, action: toolreqs.ActionAllow},
	)
	decision, err := ev.Evaluate(context.Background(), &toolreqs.ToolEvalRequest{
		ToolName: "calculator",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !decision.Allowed {
		t.Error("error in one requirement should not block the tool")
	}
	// Should have an error reason in the list.
	hasErrorReason := false
	for _, r := range decision.Reasons {
		if len(r) > 0 {
			hasErrorReason = true
		}
	}
	if !hasErrorReason {
		t.Error("expected error reason to be recorded")
	}
}

func TestEvaluator_AddRequirement(t *testing.T) {
	ev := toolreqs.NewEvaluator()
	if len(ev.Requirements()) != 0 {
		t.Fatalf("expected 0 requirements, got %d", len(ev.Requirements()))
	}
	ev.AddRequirement(&staticRequirement{kind: toolreqs.RequirementPermission, action: toolreqs.ActionAllow})
	if len(ev.Requirements()) != 1 {
		t.Fatalf("expected 1 requirement, got %d", len(ev.Requirements()))
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// staticRequirement always returns the same result.
type staticRequirement struct {
	kind         toolreqs.RequirementKind
	action       toolreqs.Action
	reason       string
	priority     int
	confirmation *toolreqs.ConfirmationRequest
}

func (r *staticRequirement) Kind() toolreqs.RequirementKind { return r.kind }
func (r *staticRequirement) Evaluate(_ context.Context, _ *toolreqs.ToolEvalRequest) (*toolreqs.RequirementResult, error) {
	return &toolreqs.RequirementResult{
		Satisfied:    r.action == toolreqs.ActionAllow,
		Action:       r.action,
		Reason:       r.reason,
		Priority:     r.priority,
		Confirmation: r.confirmation,
	}, nil
}

// errorRequirement always returns an error.
type errorRequirement struct{}

func (r *errorRequirement) Kind() toolreqs.RequirementKind { return "error" }
func (r *errorRequirement) Evaluate(_ context.Context, _ *toolreqs.ToolEvalRequest) (*toolreqs.RequirementResult, error) {
	return nil, context.DeadlineExceeded
}
