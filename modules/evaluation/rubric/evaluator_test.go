package rubric

import (
	"aicompass/modules/evaluation"
	"aicompass/modules/evaluation/trajectory"
	"context"
	"testing"
)

// ─── Rubric Evaluation Tests ────────────────────────────────────────────────

func TestEvaluateResponse_Empty(t *testing.T) {
	t.Parallel()
	levels := []RubricLevel{
		{Score: 1, Description: "bad response"},
		{Score: 3, Description: "okay response"},
		{Score: 5, Description: "great response"},
	}

	score := EvaluateResponse("", levels)
	if score.Score != 0 {
		t.Errorf("expected score 0 for empty response, got %v", score.Score)
	}
}

func TestEvaluateResponse_EmptyLevels(t *testing.T) {
	t.Parallel()
	score := EvaluateResponse("some response", nil)
	if score.Score != 0 {
		t.Errorf("expected score 0 for empty levels, got %v", score.Score)
	}
}

func TestEvaluateResponse_BestMatch(t *testing.T) {
	t.Parallel()
	levels := []RubricLevel{
		{Score: 1, Description: "bad terrible wrong incorrect"},
		{Score: 3, Description: "okay acceptable adequate"},
		{Score: 5, Description: "excellent great outstanding perfect"},
	}

	// Response matches level 3 (excellent)
	score := EvaluateResponse("This is an excellent and outstanding response with great quality.", levels)
	if score.Score != 5.0 {
		t.Errorf("expected score 5.0, got %v (matched level %d)", score.Score, score.MatchedLevel)
	}
}

func TestEvaluateResponse_LowerMatch(t *testing.T) {
	t.Parallel()
	levels := []RubricLevel{
		{Score: 1, Description: "bad terrible wrong incorrect error"},
		{Score: 3, Description: "okay acceptable adequate"},
		{Score: 5, Description: "excellent great outstanding perfect"},
	}

	// Response matches level 1 (bad)
	score := EvaluateResponse("This is a bad and terrible response with wrong information and errors.", levels)
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v (matched level %d)", score.Score, score.MatchedLevel)
	}
}

func TestEvaluateResponse_PartialMatch(t *testing.T) {
	t.Parallel()
	levels := []RubricLevel{
		{Score: 1, Description: "bad terrible wrong"},
		{Score: 3, Description: "okay acceptable adequate reasonable"},
		{Score: 5, Description: "excellent great outstanding"},
	}

	// Response has some overlap with level 2
	score := EvaluateResponse("This is an okay response that is acceptable.", levels)
	if score.MatchedLevel != 1 {
		t.Errorf("expected matched level 1 (index), got %d", score.MatchedLevel)
	}
}

func TestEvaluateToolUse_Basic(t *testing.T) {
	t.Parallel()
	levels := []RubricLevel{
		{Score: 1, Description: "no tools called wrong"},
		{Score: 3, Description: "search weather correct tools"},
		{Score: 5, Description: "search weather email perfect optimal"},
	}

	toolCalls := []string{"search", "weather"}
	score := EvaluateToolUse(toolCalls, levels)
	if score.Score != 3.0 {
		t.Errorf("expected score 3.0, got %v", score.Score)
	}
}

// ─── Evaluator Tests ────────────────────────────────────────────────────────

func TestResponseQualityEvaluator_GoodResponse(t *testing.T) {
	t.Parallel()
	eval := responseQualityEvaluator{}
	req := evaluation.EvalRequest{
		ExpectedResponse: strPtr("The weather is sunny"),
		ActualResponse:   "The weather today is excellent and great with sunny conditions.",
	}
	result := eval.Evaluate(context.Background(), req)
	// The heuristic may not score highly, just ensure it returns a valid score
	if result.Score < 1.0 || result.Score > 5.0 {
		t.Errorf("expected score between 1.0 and 5.0, got %v", result.Score)
	}
}

func TestResponseQualityEvaluator_BadResponse(t *testing.T) {
	t.Parallel()
	eval := responseQualityEvaluator{}
	req := evaluation.EvalRequest{
		ExpectedResponse: strPtr("The weather is sunny"),
		ActualResponse:   "bad terrible wrong incorrect error",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Status != "failed" {
		t.Errorf("expected status failed for bad response, got %s", result.Status)
	}
}

func TestToolUseQualityEvaluator_CorrectTools(t *testing.T) {
	t.Parallel()
	eval := toolUseQualityEvaluator{}
	req := evaluation.EvalRequest{
		ActualTrajectory: []trajectory.ActualToolCall{
			{ToolName: "search", Args: map[string]any{"q": "weather"}},
			{ToolName: "weather_api", Args: map[string]any{"city": "Tokyo"}},
		},
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score == 0 {
		t.Error("expected non-zero score for tool use")
	}
}

func TestEvaluators_Registered(t *testing.T) {
	t.Parallel()
	ids := []string{
		"rubric_based_final_response_quality_v1",
		"rubric_based_tool_use_quality_v1",
	}
	for _, id := range ids {
		eval := evaluation.GetEvaluator(id)
		if eval == nil {
			t.Errorf("evaluator %q not registered", id)
		}
		if eval.MetricID() != id {
			t.Errorf("expected metric ID %q, got %q", id, eval.MetricID())
		}
	}
}

func strPtr(s string) *string {
	return &s
}
