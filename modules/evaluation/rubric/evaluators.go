package rubric

import (
	"aicompass/modules/evaluation"
	"context"
	"encoding/json"
)

// responseQualityEvaluator implements rubric_based_final_response_quality_v1.
type responseQualityEvaluator struct{}

func (responseQualityEvaluator) MetricID() string {
	return "rubric_based_final_response_quality_v1"
}

func (responseQualityEvaluator) Evaluate(_ context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	// Default rubric for response quality if no custom rubric is provided
	defaultLevels := []RubricLevel{
		{Score: 1, Description: "response is incoherent irrelevant or completely wrong"},
		{Score: 2, Description: "response addresses topic but has major errors gaps or unclear sections"},
		{Score: 3, Description: "response is adequate covers main points with minor issues"},
		{Score: 4, Description: "response is thorough well-organized with clear explanations"},
		{Score: 5, Description: "response is exceptional comprehensive insightful and well-articulated"},
	}

	score := EvaluateResponse(req.ActualResponse, defaultLevels)
	details, _ := json.Marshal(map[string]any{
		"score":         score.Score,
		"matched_level": score.MatchedLevel,
		"level_scores":  score.LevelScores,
		"reasoning":     score.Reasoning,
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

func (toolUseQualityEvaluator) Evaluate(_ context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
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

	score := EvaluateToolUse(toolCalls, defaultLevels)
	details, _ := json.Marshal(map[string]any{
		"score":         score.Score,
		"matched_level": score.MatchedLevel,
		"level_scores":  score.LevelScores,
		"reasoning":     score.Reasoning,
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

func init() {
	evaluation.Register(responseQualityEvaluator{})
	evaluation.Register(toolUseQualityEvaluator{})
}
