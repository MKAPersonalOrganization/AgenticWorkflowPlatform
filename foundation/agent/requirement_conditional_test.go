package agent

import (
	"context"
	"testing"
)

// --------------------------------------------------------------------------
// Helper: build a history with tool calls from a compact spec
// --------------------------------------------------------------------------

// historyEntry describes one message in a test conversation.
type historyEntry struct {
	role     MessageRole
	toolName string // non-empty → assistant message with a ToolCall
}

func buildHistory(entries ...historyEntry) []*Message {
	var msgs []*Message
	for _, e := range entries {
		msg := &Message{Role: e.role}
		if e.toolName != "" {
			msg.Content = []Content{{
				Type:     ContentTypeToolCall,
				ToolCall: &ToolCall{Name: e.toolName},
			}}
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

// --------------------------------------------------------------------------
// ForceAtStep
// --------------------------------------------------------------------------

func TestConditionalRequirement_Run(t *testing.T) {
	ctx := context.Background()

	t.Run("Forced at step 1", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAtStep(1))
		inv := &Invocation{
			History: []*Message{}, // Step 1
		}

		rules, err := req.Run(ctx, inv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(rules))
		}

		rule := rules[0]
		if !rule.Forced {
			t.Error("expected tool to be forced at step 1")
		}
		if !rule.Allowed {
			t.Error("expected tool to be allowed")
		}
	})

	t.Run("Not forced at step 2", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAtStep(1))
		inv := &Invocation{
			History: []*Message{
				{Role: RoleAssistant}, // Has already responded once
			},
		}

		rules, err := req.Run(ctx, inv)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rule := rules[0]
		if rule.Forced {
			t.Error("expected tool NOT to be forced at step 2")
		}
		if !rule.Allowed {
			t.Error("expected tool to be allowed")
		}
	})

	t.Run("Only after", func(t *testing.T) {
		req := NewConditionalRequirement("search", OnlyAfter("think"))

		// Attempting to search BEFORE thinking
		invBefore := &Invocation{
			History: []*Message{},
		}
		rules1, _ := req.Run(ctx, invBefore)
		if rules1[0].Allowed {
			t.Error("expected search to NOT be allowed before think")
		}

		// Attempting to search AFTER thinking
		invAfter := &Invocation{
			History: []*Message{
				{
					Role: RoleAssistant,
					Content: []Content{
						{Type: ContentTypeToolCall, ToolCall: &ToolCall{Name: "think"}},
					},
				},
			},
		}
		rules2, _ := req.Run(ctx, invAfter)
		if !rules2[0].Allowed {
			t.Error("expected search to be allowed after think")
		}
	})

	t.Run("Max Invocations", func(t *testing.T) {
		req := NewConditionalRequirement("search", MaxInvocations(1))

		inv := &Invocation{
			History: []*Message{
				{
					Role: RoleAssistant,
					Content: []Content{
						{Type: ContentTypeToolCall, ToolCall: &ToolCall{Name: "search"}},
					},
				},
			},
		}

		rules, _ := req.Run(ctx, inv)
		if rules[0].Allowed {
			t.Error("expected search to be disallowed because max invocations reached")
		}
	})

	t.Run("Consecutive tool prevention", func(t *testing.T) {
		req := NewConditionalRequirement("search", PreventConsecutive())

		inv := &Invocation{
			History: []*Message{
				{
					Role: RoleAssistant,
					Content: []Content{
						{Type: ContentTypeToolCall, ToolCall: &ToolCall{Name: "search"}},
					},
				},
			},
		}

		rules, _ := req.Run(ctx, inv)
		if rules[0].Allowed {
			t.Error("expected search to be disallowed consecutively")
		}
	})

	t.Run("Force after", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAfter("search"))

		inv := &Invocation{
			History: []*Message{
				{
					Role: RoleAssistant,
					Content: []Content{
						{Type: ContentTypeToolCall, ToolCall: &ToolCall{Name: "search"}},
					},
				},
			},
		}

		rules, _ := req.Run(ctx, inv)
		if !rules[0].Forced {
			t.Error("expected think to be forced after search")
		}
	})
}

// --------------------------------------------------------------------------
// ForceAtStep — multi-turn with system/user/tool messages
// --------------------------------------------------------------------------

func TestConditionalRequirement_ForceAtStep_MultiTurn(t *testing.T) {
	ctx := context.Background()

	t.Run("step 1 with system and user messages", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAtStep(1))
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleSystem},
				historyEntry{role: RoleUser},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].Forced {
			t.Error("expected think forced at step 1 even with system+user prefix")
		}
	})

	t.Run("step 2 after one assistant response", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAtStep(1))
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleSystem},
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "think"},
				historyEntry{role: RoleTool}, // tool result
			),
		}
		rules, _ := req.Run(ctx, inv)
		if rules[0].Forced {
			t.Error("expected think NOT forced at step 2")
		}
	})

	t.Run("step resets on new user message", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAtStep(1))
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "think"},
				historyEntry{role: RoleTool},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
				// New user turn
				historyEntry{role: RoleUser},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].Forced {
			t.Error("expected think forced at step 1 of new user turn")
		}
	})

	t.Run("ForceAtStep 3", func(t *testing.T) {
		req := NewConditionalRequirement("calculator", ForceAtStep(3))
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "think"},
				historyEntry{role: RoleTool},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
				// We're at step 3 now
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].Forced {
			t.Error("expected calculator forced at step 3")
		}
	})
}

// --------------------------------------------------------------------------
// ForceAfterAny — ReAct pattern
// --------------------------------------------------------------------------

func TestConditionalRequirement_ForceAfterAny(t *testing.T) {
	ctx := context.Background()

	t.Run("forced after any tool (not self)", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAfterAnyTool(), PreventConsecutive())

		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].Forced {
			t.Error("expected think forced after search (ForceAfterAny)")
		}
		if !rules[0].Allowed {
			t.Error("expected think allowed when forced")
		}
	})

	t.Run("not forced after self", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAfterAnyTool(), PreventConsecutive())

		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "think"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		// With PreventConsecutive, think should NOT be allowed
		if rules[0].Allowed {
			t.Error("expected think NOT allowed consecutively")
		}
		if rules[0].Forced {
			t.Error("expected think NOT forced after self")
		}
	})

	t.Run("not forced at empty history", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAfterAnyTool())
		inv := &Invocation{
			History: []*Message{},
		}
		rules, _ := req.Run(ctx, inv)
		if rules[0].Forced {
			t.Error("expected think NOT forced with no prior tool calls")
		}
	})

	t.Run("ReAct full cycle: ForceAtStep(1) + ForceAfterAny + PreventConsecutive", func(t *testing.T) {
		req := NewConditionalRequirement("think",
			ForceAtStep(1),
			ForceAfterAnyTool(),
			PreventConsecutive(),
		)

		// Step 1: empty history → think forced
		inv1 := &Invocation{History: buildHistory(historyEntry{role: RoleUser})}
		r1, _ := req.Run(ctx, inv1)
		if !r1[0].Forced {
			t.Error("step 1: expected think forced")
		}

		// Step 2: after think → think NOT allowed (consecutive prevention)
		inv2 := &Invocation{History: buildHistory(
			historyEntry{role: RoleUser},
			historyEntry{role: RoleAssistant, toolName: "think"},
			historyEntry{role: RoleTool},
		)}
		r2, _ := req.Run(ctx, inv2)
		if r2[0].Allowed {
			t.Error("step 2: expected think NOT allowed after itself")
		}

		// Step 3: after search → think forced again (ForceAfterAny)
		inv3 := &Invocation{History: buildHistory(
			historyEntry{role: RoleUser},
			historyEntry{role: RoleAssistant, toolName: "think"},
			historyEntry{role: RoleTool},
			historyEntry{role: RoleAssistant, toolName: "search"},
			historyEntry{role: RoleTool},
		)}
		r3, _ := req.Run(ctx, inv3)
		if !r3[0].Forced {
			t.Error("step 3: expected think forced after search")
		}

		// Step 4: after think again → NOT allowed
		inv4 := &Invocation{History: buildHistory(
			historyEntry{role: RoleUser},
			historyEntry{role: RoleAssistant, toolName: "think"},
			historyEntry{role: RoleTool},
			historyEntry{role: RoleAssistant, toolName: "search"},
			historyEntry{role: RoleTool},
			historyEntry{role: RoleAssistant, toolName: "think"},
			historyEntry{role: RoleTool},
		)}
		r4, _ := req.Run(ctx, inv4)
		if r4[0].Allowed {
			t.Error("step 4: expected think NOT allowed consecutively")
		}
	})
}

// --------------------------------------------------------------------------
// MinInvocations / PreventStop
// --------------------------------------------------------------------------

func TestConditionalRequirement_MinInvocations(t *testing.T) {
	ctx := context.Background()

	t.Run("prevent stop when min not met", func(t *testing.T) {
		req := NewConditionalRequirement("search", MinInvocations(2))
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].PreventStop {
			t.Error("expected PreventStop when search invocations (1) < min (2)")
		}
	})

	t.Run("allow stop when min met", func(t *testing.T) {
		req := NewConditionalRequirement("search", MinInvocations(2))
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if rules[0].PreventStop {
			t.Error("expected NO PreventStop when min invocations met")
		}
	})
}

// --------------------------------------------------------------------------
// OnlyAfter — multiple prerequisites
// --------------------------------------------------------------------------

func TestConditionalRequirement_OnlyAfter_MultiplePrereqs(t *testing.T) {
	ctx := context.Background()

	req := NewConditionalRequirement("finalize", OnlyAfter("think", "search"))

	t.Run("blocked when no prerequisites met", func(t *testing.T) {
		inv := &Invocation{History: buildHistory(historyEntry{role: RoleUser})}
		rules, _ := req.Run(ctx, inv)
		if rules[0].Allowed {
			t.Error("expected finalize blocked (no prerequisites met)")
		}
	})

	t.Run("blocked when only one prerequisite met", func(t *testing.T) {
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "think"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if rules[0].Allowed {
			t.Error("expected finalize blocked (only think met, search missing)")
		}
	})

	t.Run("allowed when all prerequisites met", func(t *testing.T) {
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "think"},
				historyEntry{role: RoleTool},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].Allowed {
			t.Error("expected finalize allowed (all prerequisites met)")
		}
	})
}

// --------------------------------------------------------------------------
// MaxInvocations edge cases
// --------------------------------------------------------------------------

func TestConditionalRequirement_MaxInvocations_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("max 0 means never allowed", func(t *testing.T) {
		req := NewConditionalRequirement("search", MaxInvocations(0))
		inv := &Invocation{History: buildHistory(historyEntry{role: RoleUser})}
		rules, _ := req.Run(ctx, inv)
		if rules[0].Allowed {
			t.Error("expected search disallowed with max=0")
		}
	})

	t.Run("max -1 (default) means unlimited", func(t *testing.T) {
		req := NewConditionalRequirement("search") // default max = -1
		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
				historyEntry{role: RoleAssistant, toolName: "search"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].Allowed {
			t.Error("expected search allowed with unlimited max")
		}
	})

	t.Run("max invocations overrides ForceAtStep", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAtStep(1), MaxInvocations(0))
		inv := &Invocation{History: buildHistory(historyEntry{role: RoleUser})}
		rules, _ := req.Run(ctx, inv)
		if rules[0].Allowed {
			t.Error("expected MaxInvocations(0) to override ForceAtStep")
		}
		if rules[0].Forced {
			t.Error("expected tool NOT forced when max reached")
		}
	})
}

// --------------------------------------------------------------------------
// ForceAfter — specific tools
// --------------------------------------------------------------------------

func TestConditionalRequirement_ForceAfter_Specific(t *testing.T) {
	ctx := context.Background()

	t.Run("forced after one of multiple triggers", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAfter("search", "calculator"))

		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "calculator"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if !rules[0].Forced {
			t.Error("expected think forced after calculator")
		}
	})

	t.Run("NOT forced after non-trigger tool", func(t *testing.T) {
		req := NewConditionalRequirement("think", ForceAfter("search"))

		inv := &Invocation{
			History: buildHistory(
				historyEntry{role: RoleUser},
				historyEntry{role: RoleAssistant, toolName: "calculator"},
				historyEntry{role: RoleTool},
			),
		}
		rules, _ := req.Run(ctx, inv)
		if rules[0].Forced {
			t.Error("expected think NOT forced after calculator (only triggered by search)")
		}
	})
}

// --------------------------------------------------------------------------
// Combined requirements: interaction between multiple ConditionalRequirements
// --------------------------------------------------------------------------

func TestConditionalRequirement_CombinedRules(t *testing.T) {
	ctx := context.Background()

	t.Run("consecutive prevention does not break ForceAfterAny for different tools", func(t *testing.T) {
		// think: force at step 1, force after any, prevent consecutive
		thinkReq := NewConditionalRequirement("think",
			ForceAtStep(1),
			ForceAfterAnyTool(),
			PreventConsecutive(),
		)
		// search: only after think
		searchReq := NewConditionalRequirement("search", OnlyAfter("think"))

		// Step 1: only think should be forced, search blocked
		inv1 := &Invocation{History: buildHistory(historyEntry{role: RoleUser})}
		thinkRules, _ := thinkReq.Run(ctx, inv1)
		searchRules, _ := searchReq.Run(ctx, inv1)

		if !thinkRules[0].Forced {
			t.Error("step 1: think should be forced")
		}
		if searchRules[0].Allowed {
			t.Error("step 1: search should NOT be allowed (think not done yet)")
		}

		// Step 2: after think → search allowed, think blocked
		inv2 := &Invocation{History: buildHistory(
			historyEntry{role: RoleUser},
			historyEntry{role: RoleAssistant, toolName: "think"},
			historyEntry{role: RoleTool},
		)}
		thinkRules2, _ := thinkReq.Run(ctx, inv2)
		searchRules2, _ := searchReq.Run(ctx, inv2)

		if thinkRules2[0].Allowed {
			t.Error("step 2: think should NOT be allowed (consecutive)")
		}
		if !searchRules2[0].Allowed {
			t.Error("step 2: search should be allowed (think prerequisite met)")
		}

		// Step 3: after search → think forced again
		inv3 := &Invocation{History: buildHistory(
			historyEntry{role: RoleUser},
			historyEntry{role: RoleAssistant, toolName: "think"},
			historyEntry{role: RoleTool},
			historyEntry{role: RoleAssistant, toolName: "search"},
			historyEntry{role: RoleTool},
		)}
		thinkRules3, _ := thinkReq.Run(ctx, inv3)
		if !thinkRules3[0].Forced {
			t.Error("step 3: think should be forced after search")
		}
	})
}
