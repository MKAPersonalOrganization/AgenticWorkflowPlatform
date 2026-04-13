package response

import (
	"aicompass/modules/evaluation"
	"context"
	"encoding/json"
	"math"
	"strings"
)

// ─── Response Match Score (ROUGE-1 F1) ───────────────────────────────────────

type responseMatchEvaluator struct{}

func (responseMatchEvaluator) MetricID() string {
	return "response_match_score"
}

func (responseMatchEvaluator) Evaluate(_ context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	if req.ExpectedResponse == nil || *req.ExpectedResponse == "" {
		return evaluation.EvalResultOutput{
			Score:   1.0,
			Status:  "passed",
			Details: json.RawMessage(`{"note":"no expected response provided"}`),
		}
	}

	score := ROUGE1(req.ActualResponse, *req.ExpectedResponse)
	details, _ := json.Marshal(map[string]any{
		"precision": score.Precision,
		"recall":    score.Recall,
		"f1":        score.F1,
		"actual":    req.ActualResponse,
		"expected":  *req.ExpectedResponse,
	})

	status := "passed"
	if score.F1 < 0.8 {
		status = "failed"
	}

	return evaluation.EvalResultOutput{
		Score:   score.F1,
		Status:  status,
		Details: details,
	}
}

// ─── Response Evaluation Score (Coherence) ───────────────────────────────────

type responseCoherenceEvaluator struct{}

func (responseCoherenceEvaluator) MetricID() string {
	return "response_evaluation_score"
}

func (responseCoherenceEvaluator) Evaluate(_ context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	// Placeholder: In production this calls Vertex AI for coherence scoring.
	// For now, return a heuristic score based on response length and structure.
	if req.ActualResponse == "" {
		return evaluation.EvalResultOutput{
			Score:   1.0,
			Status:  "error",
			Details: json.RawMessage(`{"error":"empty response"}`),
		}
	}

	// Heuristic: score based on response quality indicators
	score := coherenceHeuristic(req.ActualResponse)
	details, _ := json.Marshal(map[string]any{
		"score":    score,
		"method":   "heuristic",
		"actual":   req.ActualResponse,
		"expected": req.ExpectedResponse,
	})

	status := "passed"
	if score < 3.0 {
		status = "failed"
	}

	return evaluation.EvalResultOutput{
		Score:   score,
		Status:  status,
		Details: details,
	}
}

// coherenceHeuristic returns a score [1, 5] based on response characteristics.
func coherenceHeuristic(response string) float64 {
	score := 2.0 // baseline

	// Length bonus
	words := len(strings.Fields(response))
	if words > 10 {
		score += 0.5
	}
	if words > 50 {
		score += 0.5
	}
	if words > 200 {
		score += 0.5
	}

	// Structure bonus (sentences, paragraphs)
	sentences := strings.Count(response, ".") + strings.Count(response, "!") + strings.Count(response, "?")
	if sentences >= 2 {
		score += 0.5
	}
	if sentences >= 5 {
		score += 0.5
	}

	return min(score, 5.0)
}

// ─── Final Response Match V2 (LLM-as-Judge) ─────────────────────────────────

type finalResponseMatchV2Evaluator struct{}

func (finalResponseMatchV2Evaluator) MetricID() string {
	return "final_response_match_v2"
}

func (finalResponseMatchV2Evaluator) Evaluate(_ context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	if req.ExpectedResponse == nil || *req.ExpectedResponse == "" {
		return evaluation.EvalResultOutput{
			Score:   1.0,
			Status:  "passed",
			Details: json.RawMessage(`{"note":"no expected response provided"}`),
		}
	}

	// Placeholder: In production this calls an LLM to judge semantic equivalence.
	// For now, use a combination of ROUGE-1 and simple semantic heuristics.
	rouge := ROUGE1(req.ActualResponse, *req.ExpectedResponse)

	// Boost score for semantic similarity indicators
	semanticBoost := semanticSimilarityHeuristic(req.ActualResponse, *req.ExpectedResponse)

	// Weighted combination: 70% ROUGE-1 + 30% semantic heuristic
	score := 0.7*rouge.F1 + 0.3*semanticBoost

	details, _ := json.Marshal(map[string]any{
		"score":          score,
		"rouge_f1":       rouge.F1,
		"semantic_score": semanticBoost,
		"method":         "hybrid",
		"actual":         req.ActualResponse,
		"expected":       *req.ExpectedResponse,
	})

	status := "passed"
	if score < 0.8 {
		status = "failed"
	}

	return evaluation.EvalResultOutput{
		Score:   roundScore(score),
		Status:  status,
		Details: details,
	}
}

// semanticSimilarityHeuristic returns a [0, 1] estimate of semantic similarity.
func semanticSimilarityHeuristic(actual, expected string) float64 {
	actualTokens := tokenize(actual)
	expectedTokens := tokenize(expected)

	if len(expectedTokens) == 0 {
		return 1.0
	}

	// Jaccard similarity of token sets
	actualSet := make(map[string]bool)
	for _, t := range actualTokens {
		actualSet[t] = true
	}

	expectedSet := make(map[string]bool)
	for _, t := range expectedTokens {
		expectedSet[t] = true
	}

	intersection := 0
	for t := range actualSet {
		if expectedSet[t] {
			intersection++
		}
	}

	union := len(actualSet) + len(expectedSet) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// ─── Registration ────────────────────────────────────────────────────────────

func init() {
	evaluation.Register(responseMatchEvaluator{})
	evaluation.Register(responseCoherenceEvaluator{})
	evaluation.Register(finalResponseMatchV2Evaluator{})
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func roundScore(s float64) float64 {
	return math.Round(s*100) / 100
}
