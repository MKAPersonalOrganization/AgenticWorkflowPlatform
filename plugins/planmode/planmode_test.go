package planmode

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/toolregistry"
	"context"
	"encoding/json"
	"testing"
)

// mustJSON marshals a value to a JSON string (test helper).
func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// parseResult unmarshals a tool result into a map.
func parseResult(t *testing.T, r agent.Result) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Content[0].(string)), &m); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}
	return m
}

// --- Provider tests ---

func TestProviderID(t *testing.T) {
	p := &Provider{}
	if got := p.ID(); got != "planmode" {
		t.Fatalf("expected 'planmode', got %q", got)
	}
}

func TestProviderDefaultSets(t *testing.T) {
	sets := (&Provider{}).DefaultSets()
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}
	if sets[0].ID != "planmode-set" {
		t.Fatalf("expected set ID 'planmode-set', got %q", sets[0].ID)
	}
	if len(sets[0].ToolIDs) != 2 {
		t.Fatalf("expected 2 tool IDs, got %d", len(sets[0].ToolIDs))
	}
}

func TestProviderDefaultTools(t *testing.T) {
	tools := (&Provider{}).DefaultTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	ids := map[string]bool{}
	for _, tool := range tools {
		ids[tool.ID] = true
	}
	if !ids["enter_plan_mode"] || !ids["exit_plan_mode"] {
		t.Fatalf("missing expected tool IDs: %v", ids)
	}
}

func TestProviderCreateExecutable(t *testing.T) {
	p := &Provider{}
	ctx := context.Background()
	var deps toolregistry.FactoryDeps

	enter, err := p.CreateExecutable(ctx, "enter_plan_mode", deps)
	if err != nil {
		t.Fatal(err)
	}
	if enter == nil {
		t.Fatal("enter_plan_mode tool is nil")
	}

	exit, err := p.CreateExecutable(ctx, "exit_plan_mode", deps)
	if err != nil {
		t.Fatal(err)
	}
	if exit == nil {
		t.Fatal("exit_plan_mode tool is nil")
	}

	unknown, err := p.CreateExecutable(ctx, "nope", deps)
	if err != nil {
		t.Fatal(err)
	}
	if unknown != nil {
		t.Fatal("expected nil for unknown tool")
	}
}

func TestProviderUIHandledTools(t *testing.T) {
	handled := (&Provider{}).UIHandledTools()
	want := map[string]bool{"enter_plan_mode": true, "exit_plan_mode": true}
	if len(handled) != len(want) {
		t.Fatalf("expected %d tools, got %d", len(want), len(handled))
	}
	for _, h := range handled {
		if !want[h] {
			t.Fatalf("unexpected tool %q", h)
		}
	}
}

// --- EnterPlanModeTool tests ---

func TestEnterPlanModeName(t *testing.T) {
	if name := (&EnterPlanModeTool{}).Name(); name != "enter_plan_mode" {
		t.Fatalf("expected 'enter_plan_mode', got %q", name)
	}
}

func TestEnterPlanModeWithReason(t *testing.T) {
	tool := &EnterPlanModeTool{}
	result, err := tool.Execute(context.Background(), agent.Arguments{
		Raw: json.RawMessage(`{"reason":"Research essay topics"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	m := parseResult(t, result)
	if m["status"] != "plan_mode_active" {
		t.Fatalf("expected status plan_mode_active, got %v", m["status"])
	}
	if m["plan_mode"] != true {
		t.Fatal("expected plan_mode=true")
	}
	if m["reason"] != "Research essay topics" {
		t.Fatalf("expected reason preserved, got %v", m["reason"])
	}
}

func TestEnterPlanModeDefaultReason(t *testing.T) {
	tool := &EnterPlanModeTool{}
	result, err := tool.Execute(context.Background(), agent.Arguments{
		Raw: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, result)
	if m["reason"] == "" {
		t.Fatal("expected a default reason")
	}
	if m["plan_mode"] != true {
		t.Fatal("expected plan_mode=true")
	}
}

func TestEnterPlanModeIncludesAllowedTools(t *testing.T) {
	tool := &EnterPlanModeTool{}
	result, _ := tool.Execute(context.Background(), agent.Arguments{
		Raw: json.RawMessage(`{}`),
	})
	m := parseResult(t, result)
	allowed, ok := m["allowed_tools"].([]any)
	if !ok || len(allowed) == 0 {
		t.Fatal("expected non-empty allowed_tools")
	}
}

// --- ExitPlanModeTool tests ---

func TestExitPlanModeName(t *testing.T) {
	if name := (&ExitPlanModeTool{}).Name(); name != "exit_plan_mode" {
		t.Fatalf("expected 'exit_plan_mode', got %q", name)
	}
}

func TestExitPlanModeValidPlan(t *testing.T) {
	plan := "## Research Plan\n\n### Objective\nHelp the student understand WWII causes.\n\n### Steps\n1. Search knowledge base for WWII materials\n2. Identify three main causes\n3. Create a structured essay outline\n4. Present sources and citations\n\n### Expected Outcome\nA well-researched outline."
	tool := &ExitPlanModeTool{}
	result, err := tool.Execute(context.Background(), agent.Arguments{
		Raw: json.RawMessage(`{"title":"WWII Essay Research","plan":` + mustJSON(plan) + `}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Error != nil {
		t.Fatalf("result error: %v", result.Error)
	}
	m := parseResult(t, result)
	if m["status"] != "plan_submitted" {
		t.Fatalf("expected plan_submitted, got %v", m["status"])
	}
	if m["plan_mode"] != false {
		t.Fatal("expected plan_mode=false")
	}
	if m["title"] != "WWII Essay Research" {
		t.Fatalf("expected title, got %v", m["title"])
	}
	steps, _ := m["steps"].(float64)
	if steps < 4 {
		t.Fatalf("expected >=4 steps, got %v", steps)
	}
}

func TestExitPlanModeEmptyPlan(t *testing.T) {
	tool := &ExitPlanModeTool{}
	result, err := tool.Execute(context.Background(), agent.Arguments{
		Raw: json.RawMessage(`{"plan":""}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil {
		t.Fatal("expected validation error for empty plan")
	}
}

func TestExitPlanModeTooBrief(t *testing.T) {
	tool := &ExitPlanModeTool{}
	result, err := tool.Execute(context.Background(), agent.Arguments{
		Raw: json.RawMessage(`{"plan":"Do it."}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Error == nil {
		t.Fatal("expected validation error for too-brief plan")
	}
}

func TestExitPlanModeDefaultTitle(t *testing.T) {
	plan := "## Objective\nAnalyze the dataset and produce visualizations.\n\n### Steps\n1. Load and clean the data\n2. Generate summary statistics\n3. Create charts\n4. Write notes"
	tool := &ExitPlanModeTool{}
	result, _ := tool.Execute(context.Background(), agent.Arguments{
		Raw: json.RawMessage(`{"plan":` + mustJSON(plan) + `}`),
	})
	m := parseResult(t, result)
	if m["title"] != "Execution Plan" {
		t.Fatalf("expected default title, got %v", m["title"])
	}
}

// --- Validation helper tests ---

func TestValidatePlan(t *testing.T) {
	cases := []struct {
		name    string
		plan    string
		wantErr bool
	}{
		{"empty", "", true},
		{"whitespace", "   \n\t  ", true},
		{"too brief", "Just do it.", true},
		{"valid", "This is a reasonably detailed plan that explains the approach we will take to solve this problem step by step.", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePlan(tc.plan)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCountPlanSteps(t *testing.T) {
	cases := []struct {
		name  string
		plan  string
		count int
	}{
		{"empty", "", 0},
		{"no steps", "Just text.", 0},
		{"numbered", "1. A\n2. B\n3. C", 3},
		{"dashes", "- A\n- B", 2},
		{"asterisks", "* A\n* B\n* C", 3},
		{"bullets", "• A\n• B", 2},
		{"mixed", "1. A\n- B\n* C\n• D", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countPlanSteps(tc.plan)
			if got != tc.count {
				t.Fatalf("expected %d, got %d", tc.count, got)
			}
		})
	}
}

// --- Interface compliance ---

func TestInterfaceCompliance(t *testing.T) {
	var _ agent.ExecutableTool = (*EnterPlanModeTool)(nil)
	var _ agent.ExecutableTool = (*ExitPlanModeTool)(nil)
	var _ toolregistry.PluginProvider = (*Provider)(nil)
}
