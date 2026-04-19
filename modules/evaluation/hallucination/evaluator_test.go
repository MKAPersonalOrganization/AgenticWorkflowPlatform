package hallucination

import (
	"aicompass/modules/evaluation"
	"context"
	"encoding/json"
	"testing"
)

// ─── Sentence Segmentation Tests ─────────────────────────────────────────────

func TestSegmentSentences(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "single sentence",
			input:    "The weather is sunny.",
			expected: 1,
		},
		{
			name:     "multiple sentences",
			input:    "The weather is sunny. It will be warm tomorrow. Don't forget sunscreen!",
			expected: 3,
		},
		{
			name:     "empty input",
			input:    "",
			expected: 0,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sentences := segmentSentences(tt.input)
			if len(sentences) != tt.expected {
				t.Errorf("expected %d sentences, got %d: %v", tt.expected, len(sentences), sentences)
			}
		})
	}
}

// ─── Classification Tests ────────────────────────────────────────────────────

func TestEvaluate_SupportedClaim(t *testing.T) {
	t.Parallel()
	response := "The weather in Tokyo is sunny with a temperature of 25 degrees."
	// Make context much longer with many overlapping tokens
	context := "Tokyo weather today is sunny with 25 degrees and light winds. " +
		"The temperature in Tokyo is 25 degrees. " +
		"Sunny conditions expected in Tokyo with light winds from the east."

	score := Evaluate(response, context)
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0 for supported claim, got %v (details: %v)", score.Score, score.Details)
	}
	if score.SupportedClaims != 1 {
		t.Errorf("expected 1 supported claim, got %d", score.SupportedClaims)
	}
	if len(score.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(score.Details))
	}
	if score.Details[0].Classification != Supported {
		t.Errorf("expected classification Supported, got %s", score.Details[0].Classification)
	}
}

func TestEvaluate_UnsupportedClaim(t *testing.T) {
	t.Parallel()
	response := "The capital of France is Barcelona and it has a population of 10 million people living in igloos."
	context := "Paris is the capital of France. The weather is mild."

	score := Evaluate(response, context)
	if score.UnsupportedClaims < 1 {
		t.Errorf("expected at least 1 unsupported claim, got %d", score.UnsupportedClaims)
	}
	if len(score.Details) < 1 {
		t.Fatalf("expected at least 1 detail, got %d", len(score.Details))
	}
	// Check that unsupported sentences are flagged
	hasUnsupported := false
	for _, d := range score.Details {
		if d.Classification == Unsupported {
			hasUnsupported = true
			break
		}
	}
	if !hasUnsupported {
		t.Error("expected at least one unsupported classification")
	}
}

func TestEvaluate_ContradictoryClaim(t *testing.T) {
	t.Parallel()
	// Use negation + high overlap with context
	response := "The weather in Tokyo is not sunny and the temperature is not 25 degrees today."
	context := "Tokyo weather today is sunny with 25 degrees and light winds. " +
		"The temperature in Tokyo is 25 degrees. " +
		"Sunny conditions expected in Tokyo with light winds from the east."

	score := Evaluate(response, context)
	if score.ContradictoryClaims < 1 {
		t.Errorf("expected at least 1 contradictory claim, got %d (details: %v)", score.ContradictoryClaims, score.Details)
	}
	// Check that contradictory sentences are flagged
	hasContradictory := false
	for _, d := range score.Details {
		if d.Classification == Contradictory {
			hasContradictory = true
			break
		}
	}
	if !hasContradictory {
		t.Error("expected at least one contradictory classification")
	}
}

func TestEvaluate_MixedClassifications(t *testing.T) {
	t.Parallel()
	response := "Tokyo weather is sunny. The capital of Mars is Barcelona. I might visit later."
	context := "Tokyo weather today: sunny, 25°C. Mars is a planet with no capital."

	score := Evaluate(response, context)
	if score.TotalClaims != 3 {
		t.Errorf("expected 3 total claims, got %d", score.TotalClaims)
	}
	// At least one should be supported (Tokyo sentence)
	if score.SupportedClaims < 1 {
		t.Errorf("expected at least 1 supported claim, got %d", score.SupportedClaims)
	}
}

func TestEvaluate_EmptyResponse(t *testing.T) {
	t.Parallel()
	score := Evaluate("", "some context")
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0 for empty response, got %v", score.Score)
	}
	if score.TotalClaims != 0 {
		t.Errorf("expected 0 total claims for empty response, got %d", score.TotalClaims)
	}
}

func TestEvaluate_QuestionsAreNotApplicable(t *testing.T) {
	t.Parallel()
	// Short question should be NotApplicable
	response := "What is the weather?"
	context := "The weather forecast shows sunny conditions."

	score := Evaluate(response, context)
	// Questions should be NotApplicable, not count against score
	hasNotApplicable := false
	for _, d := range score.Details {
		if d.Classification == NotApplicable {
			hasNotApplicable = true
			break
		}
	}
	if !hasNotApplicable {
		t.Errorf("expected questions to be classified as NotApplicable, got: %v", score.Details)
	}
}

func TestEvaluate_GreetingsAreNotApplicable(t *testing.T) {
	t.Parallel()
	response := "Hello! Thank you for your question. The weather is sunny today."
	context := "The weather forecast shows sunny conditions with a high of 25 degrees."

	score := Evaluate(response, context)
	// Greeting should be NotApplicable
	hasNotApplicable := false
	for _, d := range score.Details {
		if d.Classification == NotApplicable {
			hasNotApplicable = true
			break
		}
	}
	if !hasNotApplicable {
		t.Error("expected greeting to be classified as NotApplicable")
	}
}

func TestEvaluate_DisputedWithHedging(t *testing.T) {
	t.Parallel()
	response := "The weather might be sunny and it could rain later."
	context := "The weather forecast shows partly cloudy conditions with possible rain."

	score := Evaluate(response, context)
	// Hedging language should result in Disputed
	hasDisputed := false
	for _, d := range score.Details {
		if d.Classification == Disputed {
			hasDisputed = true
			break
		}
	}
	if !hasDisputed {
		t.Error("expected hedging language to be classified as Disputed")
	}
}

// ─── Evaluator Tests ─────────────────────────────────────────────────────────

func TestHallucinationEvaluator_SupportedResponse(t *testing.T) {
	t.Parallel()
	eval := hallucinationEvaluator{}
	expected := "Tokyo weather today: sunny, 25°C, light winds."
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "Tokyo weather is sunny with 25°C temperature and light winds.",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Score != 1.0 {
		t.Errorf("expected score 1.0, got %v", result.Score)
	}
	if result.Status != "passed" {
		t.Errorf("expected status passed, got %s", result.Status)
	}
}

func TestHallucinationEvaluator_UnsupportedResponse(t *testing.T) {
	t.Parallel()
	eval := hallucinationEvaluator{}
	expected := "Tokyo weather today: sunny, 25°C."
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "The capital of France is Barcelona and it has a population of 10 million.",
	}
	result := eval.Evaluate(context.Background(), req)
	if result.Status != "failed" {
		t.Errorf("expected status failed for unsupported response, got %s", result.Status)
	}
}

func TestHallucinationEvaluator_DetailsIncludeHighlighted(t *testing.T) {
	t.Parallel()
	eval := hallucinationEvaluator{}
	expected := "Tokyo weather today: sunny."
	req := evaluation.EvalRequest{
		ExpectedResponse: &expected,
		ActualResponse:   "The capital of Mars is Barcelona. Tokyo is sunny.",
	}
	result := eval.Evaluate(context.Background(), req)

	// Parse details JSON
	var details map[string]any
	if err := json.Unmarshal(result.Details, &details); err != nil {
		t.Fatalf("failed to unmarshal details: %v", err)
	}

	sentences, ok := details["sentences"].([]any)
	if !ok {
		t.Fatal("expected sentences array in details")
	}

	// Check that at least one sentence is highlighted
	hasHighlighted := false
	for _, s := range sentences {
		sentence, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if highlighted, ok := sentence["highlighted"].(bool); ok && highlighted {
			hasHighlighted = true
			break
		}
	}
	if !hasHighlighted {
		t.Error("expected at least one highlighted sentence in details")
	}
}

func TestHallucinationEvaluator_NoExpectedResponse(t *testing.T) {
	t.Parallel()
	eval := hallucinationEvaluator{}
	req := evaluation.EvalRequest{
		ExpectedResponse: nil,
		ActualResponse:   "any response at all",
	}
	result := eval.Evaluate(context.Background(), req)
	// Without context, everything is unsupported, score should be 0
	if result.Score != 0.0 {
		t.Errorf("expected score 0.0 without context, got %v", result.Score)
	}
}

func TestEvaluators_Registered(t *testing.T) {
	t.Parallel()
	// Registration is driven by main.go in production. Test the contract
	// directly with a local registrar so we don't depend on global state
	// (which would otherwise require a TestMain hook just to populate it).
	var registered []evaluation.Evaluator
	RegisterEvaluators(func(e evaluation.Evaluator) {
		registered = append(registered, e)
	})
	var got evaluation.Evaluator
	for _, e := range registered {
		if e.MetricID() == "hallucinations_v1" {
			got = e
			break
		}
	}
	if got == nil {
		t.Fatal("hallucinations_v1 evaluator not registered by RegisterEvaluators")
	}
	if got.MetricID() != "hallucinations_v1" {
		t.Errorf("expected metric ID hallucinations_v1, got %s", got.MetricID())
	}
}

// ─── Content Word Tests ─────────────────────────────────────────────────────

func TestIsContentWord(t *testing.T) {
	t.Parallel()
	if IsContentWord("the") {
		t.Error("expected 'the' to not be a content word")
	}
	if IsContentWord("weather") {
		// This should be true
	} else {
		t.Error("expected 'weather' to be a content word")
	}
	if IsContentWord("a") {
		t.Error("expected 'a' to not be a content word")
	}
}

func TestHasMeaningfulContent(t *testing.T) {
	t.Parallel()
	if HasMeaningfulContent("the and for are") {
		t.Error("expected stop words only to not have meaningful content")
	}
	if !HasMeaningfulContent("Tokyo weather sunny 25 degrees") {
		t.Error("expected substantive text to have meaningful content")
	}
}
