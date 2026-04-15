package hallucination

import (
	"aicompass/modules/evaluation"
	"context"
	"encoding/json"
)

// hallucinationEvaluator implements the Evaluator interface for hallucination detection.
type hallucinationEvaluator struct{}

func (hallucinationEvaluator) MetricID() string {
	return "hallucinations_v1"
}

func (hallucinationEvaluator) Evaluate(_ context.Context, req evaluation.EvalRequest) evaluation.EvalResultOutput {
	// Use expected response as context for checking actual response
	context := ""
	if req.ExpectedResponse != nil {
		context = *req.ExpectedResponse
	}

	score := Evaluate(req.ActualResponse, context)

	// Build details with highlighting for unsupported/contradictory
	type SentenceDetail struct {
		Sentence       string         `json:"sentence"`
		Classification Classification `json:"classification"`
		Confidence     float64        `json:"confidence"`
		Highlighted    bool           `json:"highlighted"` // red if unsupported/contradictory
	}

	details := make([]SentenceDetail, 0, len(score.Details))
	for _, d := range score.Details {
		details = append(details, SentenceDetail{
			Sentence:       d.Sentence,
			Classification: d.Classification,
			Confidence:     d.Confidence,
			Highlighted:    d.Classification == Unsupported || d.Classification == Contradictory,
		})
	}

	detailsJSON, _ := json.Marshal(map[string]any{
		"score":                score.Score,
		"total_claims":         score.TotalClaims,
		"supported_claims":     score.SupportedClaims,
		"unsupported_claims":   score.UnsupportedClaims,
		"contradictory_claims": score.ContradictoryClaims,
		"disputed_claims":      score.DisputedClaims,
		"sentences":            details,
	})

	status := "passed"
	if score.Score < 0.8 {
		status = "failed"
	}

	return evaluation.EvalResultOutput{
		Score:   score.Score,
		Status:  status,
		Details: detailsJSON,
	}
}

func RegisterEvaluators(register func(evaluation.Evaluator)) {
	register(hallucinationEvaluator{})
}
