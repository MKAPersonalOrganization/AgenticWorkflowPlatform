package rubric

import (
	"aicompass/modules/evaluation"
	"aicompass/modules/evaluation/llmjudge"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// responseQualityEvaluator implements rubric_based_final_response_quality_v1.
type responseQualityEvaluator struct{}

func (responseQualityEvaluator) MetricID() string {
	return "rubric_based_final_response_quality_v1"
}

func (responseQualityEvaluator) Evaluate(ctx context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	// Default rubric for response quality if no custom rubric is provided
	defaultLevels := []RubricLevel{
		{Score: 1, Description: "response is incoherent irrelevant or completely wrong"},
		{Score: 2, Description: "response addresses topic but has major errors gaps or unclear sections"},
		{Score: 3, Description: "response is adequate covers main points with minor issues"},
		{Score: 4, Description: "response is thorough well-organized with clear explanations"},
		{Score: 5, Description: "response is exceptional comprehensive insightful and well-articulated"},
	}

	var score RubricScore
	var method string

	// Use LLM judge if available; otherwise use heuristic
	if req.JudgeConfig != nil && req.JudgeConfig.APIKey != "" {
		score, method = evaluateResponseWithLLMJudge(ctx, req.ActualResponse, defaultLevels, req.JudgeConfig)
	} else {
		score = EvaluateResponse(req.ActualResponse, defaultLevels)
		method = "heuristic"
	}

	details, _ := json.Marshal(map[string]any{
		"score":         score.Score,
		"matched_level": score.MatchedLevel,
		"level_scores":  score.LevelScores,
		"reasoning":     score.Reasoning,
		"method":        method,
		"actual":        req.ActualResponse,
		"expected":      req.ExpectedResponse,
	})

	status := "passed"
	if score.Score < 3.0 {
		status = "failed"
	}

	return evaluation.EvalResultOutput{
		Score:   score.Score,
		Status:  status,
		Details: details,
	}
}

// toolUseQualityEvaluator implements rubric_based_tool_use_quality_v1.
type toolUseQualityEvaluator struct{}

func (toolUseQualityEvaluator) MetricID() string {
	return "rubric_based_tool_use_quality_v1"
}

func (toolUseQualityEvaluator) Evaluate(ctx context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	// Default rubric for tool use quality
	defaultLevels := []RubricLevel{
		{Score: 1, Description: "no tools called wrong"},
		{Score: 3, Description: "search weather correct tools"},
		{Score: 5, Description: "search weather email perfect optimal"},
	}

	// Build tool call descriptions
	toolCalls := make([]string, 0, len(req.ActualTrajectory))
	for _, tc := range req.ActualTrajectory {
		toolCalls = append(toolCalls, tc.ToolName)
	}

	var score RubricScore
	var method string
	response := strings.Join(toolCalls, " ")

	// Use LLM judge if available; otherwise use heuristic
	if req.JudgeConfig != nil && req.JudgeConfig.APIKey != "" {
		score, method = evaluateResponseWithLLMJudge(ctx, response, defaultLevels, req.JudgeConfig)
	} else {
		score = EvaluateToolUse(toolCalls, defaultLevels)
		method = "heuristic"
	}

	details, _ := json.Marshal(map[string]any{
		"score":         score.Score,
		"matched_level": score.MatchedLevel,
		"level_scores":  score.LevelScores,
		"reasoning":     score.Reasoning,
		"method":        method,
		"tool_calls":    toolCalls,
	})

	status := "passed"
	if score.Score < 3.0 {
		status = "failed"
	}

	return evaluation.EvalResultOutput{
		Score:   score.Score,
		Status:  status,
		Details: details,
	}
}

// ─── LLM Judge Helper ────────────────────────────────────────────────────────

// evaluateResponseWithLLMJudge calls the Gemini API to score a response against rubric levels.
// Returns (score, method).
func evaluateResponseWithLLMJudge(ctx context.Context, response string, levels []RubricLevel, judgeConfig *evaluation.JudgeConfig) (RubricScore, string) {
	model := "gemini-2.5-flash"
	if judgeConfig.Model != "" {
		model = judgeConfig.Model
	}

	judge := llmjudge.New(judgeConfig.APIKey, model)

	// Build rubric description
	rubricText := "Evaluate the response against the following rubric levels:\n\n"
	for i, level := range levels {
		rubricText += fmt.Sprintf("Level %d (Score %0.0f): %s\n", i+1, level.Score, level.Description)
	}

	// Format prompt
	prompt := fmt.Sprintf(`You are an expert evaluator. Your task is to score the given response against rubric levels.

Response to evaluate:
%s

%s

For each rubric level, determine if the response matches that level. Output your verdict in the following format:
Level 1: [yes or no]
Level 2: [yes or no]
Level 3: [yes or no]
...

Output only the verdicts, one per line.`, response, rubricText)

	resp, err := judge.Score(ctx, prompt)
	if err != nil {
		// Fallback to heuristic on error
		return EvaluateResponse(response, levels), "heuristic_fallback"
	}

	// Parse verdicts line by line
	levelScores := make([]float64, len(levels))
	lines := strings.Split(resp, "\n")
	bestLevel := 0
	bestScore := 0.0

	for i := 0; i < len(levels); i++ {
		verdict := 0.0
		if i < len(lines) {
			line := strings.ToLower(lines[i])
			if strings.Contains(line, "yes") {
				verdict = 1.0
			}
		}
		levelScores[i] = verdict
		if verdict > 0 && levels[i].Score > bestScore {
			bestScore = levels[i].Score
			bestLevel = i
		}
	}

	// If no level matched "yes", use best heuristic match as fallback
	if bestScore == 0 {
		return EvaluateResponse(response, levels), "heuristic_fallback"
	}

	return RubricScore{
		Score:        levels[bestLevel].Score,
		MatchedLevel: bestLevel,
		LevelScores:  levelScores,
		Reasoning:    fmt.Sprintf("LLM judge: matched level %d", bestLevel+1),
	}, "llm_judge"
}

func RegisterEvaluators(register func(evaluation.Evaluator)) {
	register(responseQualityEvaluator{})
	register(toolUseQualityEvaluator{})
}
