package response

import (
	"aicompass/modules/evaluation"
	"context"
	"testing"
)

// ─── ROUGE-1 Tests ───────────────────────────────────────────────────────────

func TestROUGE1_Identical(t *testing.T) {
	t.Parallel()
	score := ROUGE1("the weather is sunny", "the weather is sunny")
	if score.F1 != 1.0 {
		t.Errorf("expected F1 1.0, got %v", score.F1)
	}
	if score.Precision != 1.0 {
		t.Errorf("expected precision 1.0, got %v", score.Precision)
	}
	if score.Recall != 1.0 {
		t.Errorf("expected recall 1.0, got %v", score.Recall)
	}
}

func TestROUGE1_CompletelyDifferent(t *testing.T) {
	t.Parallel()
	score := ROUGE1("the weather is sunny", "I like to eat pizza")
	if score.F1 != 0.0 {
		t.Errorf("expected F1 0.0, got %v", score.F1)
	}
}

func TestROUGE1_PartialOverlap(t *testing.T) {
	t.Parallel()
	// 3 overlapping words out of 5 candidate, 4 reference
	score := ROUGE1("the weather is sunny today", "the weather was rainy yesterday")
	if score.F1 <= 0.0 {
		t.Errorf("expected F1 > 0.0, got %v", score.F1)
	}
	if score.F1 >= 1.0 {
		t.Errorf("expected F1 < 1.0, got %v", score.F1)
	}
}

func TestROUGE1_EmptyCandidate(t *testing.T) {
	t.Parallel()
	score := ROUGE1("", "the weather is sunny")
	if score.F1 != 0.0 {
		t.Errorf("expected F1 0.0, got %v", score.F1)
	}
}

func TestROUGE1_EmptyReference(t *testing.T) {
	t.Parallel()
	score := ROUGE1("the weather is sunny", "")
	if score.F1 != 0.0 {
		t.Errorf("expected F1 0.0, got %v", score.F1)
	}
}

func TestROUGE1_BothEmpty(t *testing.T) {
	t.Parallel()
	score := ROUGE1("", "")
	if score.F1 != 1.0 {
		t.Errorf("expected F1 1.0 for both empty, got %v", score.F1)
	}
}

func TestROUGE1_PunctuationHandling(t *testing.T) {
	t.Parallel()
	score := ROUGE1("Hello, world!", "hello world")
	if score.F1 != 1.0 {
		t.Errorf("expected F1 1.0 (punctuation ignored), got %v", score.F1)
	}
}

func TestROUGE1_CaseInsensitive(t *testing.T) {
	t.Parallel()
	score := ROUGE1("The Weather Is Sunny", "the weather is sunny")
	if score.F1 != 1.0 {
		t.Errorf("expected F1 1.0 (case insensitive), got %v", score.F1)
	}
}

func TestROUGE1_DifferentLengths(t *testing.T) {
	t.Parallel()
	// Short candidate vs long reference
	score := ROUGE1("sunny weather", "the weather today is very sunny and beautiful")
	if score.F1 <= 0.0 {
		t.Errorf("expected F1 > 0.0, got %v", score.F1)
	}
	// Precision should be higher than recall (all candidate tokens in reference)
	if score.Precision < score.Recall {
		t.Errorf("expected precision >= recall for short candidate, got p=%v r=%v", score.Precision, score.Recall)
	}
}

// ─── Evaluator Tests ─────────────────────────────────────────────────────────

func TestResponseMatchEvaluator_Identical(t *testing.T) {
	t.Parallel()
	eval := responseMatchEvaluator{}
	expected := "the weather is sunny"
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "the weather is sunny",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", result.Score)
	}
	if result.Status != "passed" {
		t.Errorf("expected status passed, got %s", result.Status)
	}
}

func TestResponseMatchEvaluator_Different(t *testing.T) {
	t.Parallel()
	eval := responseMatchEvaluator{}
	expected := "the weather is sunny"
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "I like to eat pizza",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score != 0.0 {
		t.Errorf("expected score 0.0, got %v", result.Score)
	}
	if result.Status != "failed" {
		t.Errorf("expected status failed, got %s", result.Status)
	}
}

func TestResponseMatchEvaluator_NoExpected(t *testing.T) {
	t.Parallel()
	eval := responseMatchEvaluator{}
	req := evaluation.EvalRequest{
		ExpectedResponse: nil,
		ActualResponse:   "any response",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0 when no expected, got %v", result.Score)
	}
}

func TestCoherenceEvaluator_EmptyResponse(t *testing.T) {
	t.Parallel()
	eval := responseCoherenceEvaluator{}
	req := evaluation.EvalRequest{
		ActualResponse: "",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Status != "error" {
		t.Errorf("expected status error for empty response, got %s", result.Status)
	}
}

func TestCoherenceEvaluator_LongResponse(t *testing.T) {
	t.Parallel()
	eval := responseCoherenceEvaluator{}
	req := evaluation.EvalRequest{
		ActualResponse: "This is a longer response with multiple sentences. It should score higher because it has more content and structure. The weather today is very interesting indeed.",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score < 2.0 {
		t.Errorf("expected score >= 2.0 for long response, got %v", result.Score)
	}
}

func TestFinalResponseMatchV2Evaluator_Identical(t *testing.T) {
	t.Parallel()
	eval := finalResponseMatchV2Evaluator{}
	expected := "the weather is sunny"
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "the weather is sunny",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", result.Score)
	}
}

func TestFinalResponseMatchV2Evaluator_Different(t *testing.T) {
	t.Parallel()
	eval := finalResponseMatchV2Evaluator{}
	expected := "the weather is sunny"
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "I like to eat pizza for dinner every day",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Status != "failed" {
		t.Errorf("expected status failed, got %s", result.Status)
	}
}

func TestFinalResponseMatchV2Evaluator_PartialSimilarity(t *testing.T) {
	t.Parallel()
	eval := finalResponseMatchV2Evaluator{}
	expected := "The weather today is sunny with a high of 25 degrees."
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "Today's weather is sunny, reaching 25 degrees Celsius.",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score <= 0.0 {
		t.Errorf("expected score > 0.0 for partial similarity, got %v", result.Score)
	}
}

func TestEvaluators_Registered(t *testing.T) {
	t.Parallel()
	// Use a local registrar — see hallucination/evaluator_test.go for why.
	registry := map[string]evaluation.Evaluator{}
	RegisterEvaluators(func(e evaluation.Evaluator) {
		registry[e.MetricID()] = e
	})
	ids := []string{"response_match_score", "response_evaluation_score", "final_response_match_v2"}
	for _, id := range ids {
		eval, ok := registry[id]
		if !ok {
			t.Errorf("evaluator %q not registered by RegisterEvaluators", id)
			continue
		}
		if eval.MetricID() != id {
			t.Errorf("expected metric ID %q, got %q", id, eval.MetricID())
		}
	}
}
