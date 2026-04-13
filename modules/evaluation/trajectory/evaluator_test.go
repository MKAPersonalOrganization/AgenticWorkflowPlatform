package trajectory

import (
	"testing"
)

// ─── EXACT Match Tests ───────────────────────────────────────────────────────

func TestExact_Identical(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", score.Score)
	}
	if score.Matched != 2 {
		t.Errorf("expected 2 matched, got %d", score.Matched)
	}
}

func TestExact_CompletelyDifferent(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
	}
	actual := []ActualToolCall{
		{ToolName: "calculator", Args: map[string]any{"expr": "2+2"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 0.0 {
		t.Errorf("expected score 0.0, got %v", score.Score)
	}
	// Tool exists at position 0 but doesn't match → tracked in details/outOfOrder, not missing
	if len(score.OutOfOrder) != 1 {
		t.Errorf("expected 1 out_of_order, got %d: %v", len(score.OutOfOrder), score.OutOfOrder)
	}
	if len(score.Missing) != 0 {
		t.Errorf("expected 0 missing (position exists, just wrong tool), got %d", len(score.Missing))
	}
}

func TestExact_PartialMatch(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
		{ToolName: "email", Args: map[string]any{"to": "user@example.com"}},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "weather_api", Args: map[string]any{"city": "Paris"}}, // different arg
		{ToolName: "email", Args: map[string]any{"to": "user@example.com"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 0.67 {
		t.Errorf("expected score 0.67, got %v", score.Score)
	}
	if score.Matched != 2 {
		t.Errorf("expected 2 matched, got %d", score.Matched)
	}
}

// ─── IN_ORDER Match Tests ────────────────────────────────────────────────────

func TestInOrder_WithInterleavedExtras(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", MatchType: MatchInOrder},
		{ToolName: "weather_api", MatchType: MatchInOrder},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "cache_lookup", Args: map[string]any{"key": "x"}}, // extra
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", score.Score)
	}
	if len(score.Extra) == 0 {
		t.Error("expected extra tool 'cache_lookup' to be reported")
	}
}

func TestInOrder_MissingTool(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", MatchType: MatchInOrder},
		{ToolName: "weather_api", MatchType: MatchInOrder},
		{ToolName: "email", MatchType: MatchInOrder},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "email", Args: map[string]any{"to": "user@example.com"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 0.67 {
		t.Errorf("expected score 0.67, got %v", score.Score)
	}
	if len(score.Missing) != 1 || score.Missing[0] != "weather_api" {
		t.Errorf("expected missing [weather_api], got %v", score.Missing)
	}
}

func TestInOrder_WrongOrder(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", MatchType: MatchInOrder},
		{ToolName: "weather_api", MatchType: MatchInOrder},
	}
	actual := []ActualToolCall{
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
	}
	score := Evaluate(expected, actual)
	// search is found at position 1, weather_api can't be found after that
	if score.Score != 0.5 {
		t.Errorf("expected score 0.5, got %v", score.Score)
	}
}

// ─── ANY_ORDER Match Tests ───────────────────────────────────────────────────

func TestAnyOrder_AllPresentDifferentOrder(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", MatchType: MatchAnyOrder},
		{ToolName: "weather_api", MatchType: MatchAnyOrder},
		{ToolName: "email", MatchType: MatchAnyOrder},
	}
	actual := []ActualToolCall{
		{ToolName: "email", Args: map[string]any{"to": "user@example.com"}},
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", score.Score)
	}
}

func TestAnyOrder_SomeMissing(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", MatchType: MatchAnyOrder},
		{ToolName: "weather_api", MatchType: MatchAnyOrder},
		{ToolName: "email", MatchType: MatchAnyOrder},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 0.67 {
		t.Errorf("expected score 0.67, got %v", score.Score)
	}
	if len(score.Missing) != 1 {
		t.Errorf("expected 1 missing, got %d", len(score.Missing))
	}
}

func TestAnyOrder_DuplicateCalls(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", MatchType: MatchAnyOrder},
		{ToolName: "search", MatchType: MatchAnyOrder},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "search", Args: map[string]any{"q": "forecast"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", score.Score)
	}
}

// ─── Edge Cases ──────────────────────────────────────────────────────────────

func TestEmptyExpected_ReturnsFullScore(t *testing.T) {
	t.Parallel()
	score := Evaluate(nil, []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
	})
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0 for empty expected, got %v", score.Score)
	}
	if score.Total != 0 {
		t.Errorf("expected total 0, got %d", score.Total)
	}
}

func TestEmptyActual_ReturnsZero(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
	}
	score := Evaluate(expected, nil)
	if score.Score != 0.0 {
		t.Errorf("expected score 0.0, got %v", score.Score)
	}
	if len(score.Missing) != 1 {
		t.Errorf("expected 1 missing, got %d", len(score.Missing))
	}
}

func TestExtraToolsIdentified(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search", MatchType: MatchInOrder},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "weather"}},
		{ToolName: "cache", Args: map[string]any{"key": "x"}},
		{ToolName: "log", Args: map[string]any{"msg": "done"}},
	}
	score := Evaluate(expected, actual)
	if len(score.Extra) != 2 {
		t.Errorf("expected 2 extra tools, got %d: %v", len(score.Extra), score.Extra)
	}
}

func TestParameterMismatchDetected(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo", "units": "celsius"}},
	}
	actual := []ActualToolCall{
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo", "units": "fahrenheit"}},
	}
	score := Evaluate(expected, actual)
	if score.Score != 0.0 {
		t.Errorf("expected score 0.0 (arg mismatch), got %v", score.Score)
	}
	// But param_match should be partial
	if len(score.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(score.Details))
	}
	if score.Details[0].ParamMatch != 0.5 {
		t.Errorf("expected param_match 0.5, got %v", score.Details[0].ParamMatch)
	}
}

func TestScoreRounded(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "a"},
		{ToolName: "b"},
		{ToolName: "c"},
	}
	actual := []ActualToolCall{
		{ToolName: "a"},
		{ToolName: "b"},
	}
	score := Evaluate(expected, actual)
	// 2/3 = 0.666... → rounds to 0.67
	if score.Score != 0.67 {
		t.Errorf("expected score 0.67, got %v", score.Score)
	}
}

// ─── Details Completeness ────────────────────────────────────────────────────

func TestDetails_ContainsExpectedAndActual(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search"},
		{ToolName: "weather_api"},
	}
	actual := []ActualToolCall{
		{ToolName: "search", Args: map[string]any{"q": "test"}},
		{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
	}
	score := Evaluate(expected, actual)
	if len(score.Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(score.Details))
	}
	for i, d := range score.Details {
		if !d.Expected {
			t.Errorf("detail %d: expected=true", i)
		}
		if !d.Actual {
			t.Errorf("detail %d: actual=true", i)
		}
	}
}

func TestDetails_MissingToolMarkedCorrectly(t *testing.T) {
	t.Parallel()
	expected := []ExpectedToolCall{
		{ToolName: "search"},
		{ToolName: "missing_tool"},
	}
	actual := []ActualToolCall{
		{ToolName: "search"},
	}
	score := Evaluate(expected, actual)
	if len(score.Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(score.Details))
	}
	if score.Details[1].Actual {
		t.Error("detail for missing tool should have actual=false")
	}
	if score.Details[1].Matched {
		t.Error("detail for missing tool should have matched=false")
	}
}
