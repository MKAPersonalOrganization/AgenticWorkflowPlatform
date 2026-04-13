package evaluation

import (
	"aicompass/modules/evaluation/trajectory"
	"context"
	"encoding/json"
	"fmt"
)

// EvalRequest is the input to an evaluator for a single case/invocation.
type EvalRequest struct {
	ExpectedTrajectory json.RawMessage
	ExpectedResponse   *string
	ActualTrajectory   []trajectory.ActualToolCall
	ActualResponse     string
}

// EvalResultOutput is the output from an evaluator.
type EvalResultOutput struct {
	Score   float64         `json:"score"`
	Status  string          `json:"status"` // passed, failed, error
	Details json.RawMessage `json:"details,omitempty"`
}

// Evaluator is the interface all metric evaluators must implement.
type Evaluator interface {
	Evaluate(ctx context.Context, req EvalRequest) EvalResultOutput
	MetricID() string
}

// ─── Evaluator Registry ──────────────────────────────────────────────────────

var registry = make(map[string]Evaluator)

// Register adds an evaluator to the global registry.
func Register(e Evaluator) {
	registry[e.MetricID()] = e
}

// GetEvaluator returns an evaluator by metric ID, or nil if not found.
func GetEvaluator(metricID string) Evaluator {
	return registry[metricID]
}

// ListEvaluators returns all registered evaluator metric IDs.
func ListEvaluators() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	return ids
}

// ─── Trajectory Evaluator (registered on init) ──────────────────────────────

type trajectoryEvaluator struct{}

func (trajectoryEvaluator) MetricID() string {
	return "tool_trajectory_avg_score"
}

func (trajectoryEvaluator) Evaluate(_ context.Context, req EvalRequest) EvalResultOutput {
	if len(req.ExpectedTrajectory) == 0 {
		return EvalResultOutput{Score: 1.0, Status: "passed", Details: nil}
	}

	var expected []trajectory.ExpectedToolCall
	if err := json.Unmarshal(req.ExpectedTrajectory, &expected); err != nil {
		return EvalResultOutput{
			Score:   0,
			Status:  "error",
			Details: json.RawMessage(fmt.Sprintf(`{"error":"invalid expected_trajectory: %s"}`, err.Error())),
		}
	}

	score := trajectory.Evaluate(expected, req.ActualTrajectory)
	details, _ := json.Marshal(score)

	status := "passed"
	if score.Score < 0.8 {
		status = "failed"
	}

	return EvalResultOutput{
		Score:   score.Score,
		Status:  status,
		Details: details,
	}
}

func init() {
	Register(trajectoryEvaluator{})
}
