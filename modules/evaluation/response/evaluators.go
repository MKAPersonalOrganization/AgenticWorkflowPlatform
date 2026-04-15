package response

import (
	"aicompass/modules/evaluation"
	"aicompass/modules/evaluation/llmjudge"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
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

func (finalResponseMatchV2Evaluator) Evaluate(ctx context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	if req.ExpectedResponse == nil || *req.ExpectedResponse == "" {
		return evaluation.EvalResultOutput{
			Score:   1.0,
			Status:  "passed",
			Details: json.RawMessage(`{"note":"no expected response provided"}`),
		}
	}

	var score float64
	var details map[string]any

	// If JudgeConfig is available, use LLM judge; otherwise fall back to heuristic
	if req.JudgeConfig != nil && req.JudgeConfig.APIKey != "" {
		judgeScore, _, judgeDetails := evaluateWithLLMJudge(ctx, req)
		score = judgeScore
		details = judgeDetails
	} else {
		// Fallback: heuristic combination (70% ROUGE-1 + 30% semantic)
		rouge := ROUGE1(req.ActualResponse, *req.ExpectedResponse)
		semanticBoost := semanticSimilarityHeuristic(req.ActualResponse, *req.ExpectedResponse)
		score = 0.7*rouge.F1 + 0.3*semanticBoost
		details = map[string]any{
			"score":          score,
			"rouge_f1":       rouge.F1,
			"semantic_score": semanticBoost,
			"actual":         req.ActualResponse,
			"expected":       *req.ExpectedResponse,
		}
	}

	detailsJSON, _ := json.Marshal(details)

	status := "passed"
	if score < 0.8 {
		status = "failed"
	}

	return evaluation.EvalResultOutput{
		Score:   roundScore(score),
		Status:  status,
		Details: detailsJSON,
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

// ─── LLM Judge Helper ────────────────────────────────────────────────────────

// evaluateWithLLMJudge calls the Gemini API to judge semantic equivalence.
// Returns (score, method, details).
func evaluateWithLLMJudge(ctx context.Context, req evaluation.EvalRequest) (float64, string, map[string]any) {
	model := "gemini-2.5-flash"
	if req.JudgeConfig.Model != "" {
		model = req.JudgeConfig.Model
	}

	judge := llmjudge.New(req.JudgeConfig.APIKey, model)

	// Format prompt using ADK Python template
	prompt := fmt.Sprintf(`You are an expert rater for an AI agent's response to a user prompt. Your task is to evaluate whether the agent's response is semantically valid and matches the reference response.

User prompt: %s

Agent response: %s

Reference response: %s

Provide your evaluation in the following JSON format:
{"reasoning": "[brief explanation of your assessment]", "is_the_agent_response_valid": "[valid or invalid]"}

Output only the JSON, no other text.`, req.ActualResponse, *req.ExpectedResponse, req.ActualResponse)

	response, err := judge.Score(ctx, prompt)
	if err != nil {
		// Fallback to heuristic on error
		rouge := ROUGE1(req.ActualResponse, *req.ExpectedResponse)
		semanticBoost := semanticSimilarityHeuristic(req.ActualResponse, *req.ExpectedResponse)
		score := 0.7*rouge.F1 + 0.3*semanticBoost
		return score, "hybrid_heuristic_fallback", map[string]any{
			"score":           score,
			"rouge_f1":        rouge.F1,
			"semantic_score":  semanticBoost,
			"judge_error":     err.Error(),
			"actual":          req.ActualResponse,
			"expected":        *req.ExpectedResponse,
		}
	}

	// Parse response to extract "is_the_agent_response_valid"
	// Pattern: "is_the_agent_response_valid": "[value]"
	re := regexp.MustCompile(`"is_the_agent_response_valid":\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(response)

	var score float64
	reasoning := ""

	if len(matches) > 1 {
		validity := strings.ToLower(strings.TrimSpace(matches[1]))
		if validity == "valid" {
			score = 1.0
		} else {
			score = 0.0
		}

		// Try to extract reasoning too
		reasoningRe := regexp.MustCompile(`"reasoning":\s*"([^"]+)"`)
		reasoningMatches := reasoningRe.FindStringSubmatch(response)
		if len(reasoningMatches) > 1 {
			reasoning = reasoningMatches[1]
		}
	} else {
		// Could not parse response, treat as invalid
		score = 0.0
		reasoning = "Could not parse judge response"
	}

	return score, "llm_judge", map[string]any{
		"score":       score,
		"reasoning":   reasoning,
		"judge_model": model,
		"actual":      req.ActualResponse,
		"expected":    *req.ExpectedResponse,
	}
}

// ─── Registration ────────────────────────────────────────────────────────────

func RegisterEvaluators(register func(evaluation.Evaluator)) {
	register(responseMatchEvaluator{})
	register(responseCoherenceEvaluator{})
	register(finalResponseMatchV2Evaluator{})
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
