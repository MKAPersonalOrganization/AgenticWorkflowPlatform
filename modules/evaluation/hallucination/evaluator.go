// Package hallucination implements hallucination detection evaluators
// for the evaluation framework.
package hallucination

import (
	"regexp"
	"strings"
)

// Classification represents the result of checking a claim against context.
type Classification string

const (
	Supported     Classification = "supported"
	Unsupported   Classification = "unsupported"
	Contradictory Classification = "contradictory"
	Disputed      Classification = "disputed"
	NotApplicable Classification = "not_applicable"
)

// ClaimAnalysis is the result of analyzing a single sentence/claim.
type ClaimAnalysis struct {
	Sentence       string         `json:"sentence"`
	Classification Classification `json:"classification"`
	Confidence     float64        `json:"confidence"` // 0.0 - 1.0
}

// HallucinationScore is the overall result of hallucination evaluation.
type HallucinationScore struct {
	Score               float64         `json:"score"` // proportion of supported claims [0, 1]
	TotalClaims         int             `json:"total_claims"`
	SupportedClaims     int             `json:"supported_claims"`
	UnsupportedClaims   int             `json:"unsupported_claims"`
	ContradictoryClaims int             `json:"contradictory_claims"`
	DisputedClaims      int             `json:"disputed_claims"`
	Details             []ClaimAnalysis `json:"details"`
}

// Evaluate analyzes a response for hallucinations against the expected context.
func Evaluate(response, context string) HallucinationScore {
	sentences := segmentSentences(response)
	if len(sentences) == 0 {
		return HallucinationScore{Score: 1.0}
	}

	contextTokens := tokenize(context)
	details := make([]ClaimAnalysis, 0, len(sentences))
	supported := 0
	unsupported := 0
	contradictory := 0
	disputed := 0

	for _, sentence := range sentences {
		analysis := classifySentence(sentence, contextTokens)
		details = append(details, analysis)

		switch analysis.Classification {
		case Supported:
			supported++
		case Unsupported:
			unsupported++
		case Contradictory:
			contradictory++
		case Disputed:
			disputed++
		}
	}

	score := float64(supported) / float64(len(sentences))
	return HallucinationScore{
		Score:               roundScore(score),
		TotalClaims:         len(sentences),
		SupportedClaims:     supported,
		UnsupportedClaims:   unsupported,
		ContradictoryClaims: contradictory,
		DisputedClaims:      disputed,
		Details:             details,
	}
}

// segmentSentences splits text into sentences based on punctuation.
func segmentSentences(text string) []string {
	// Find all sentence segments including their ending punctuation
	re := regexp.MustCompile(`[^.!?]*[.!?]+`)
	matches := re.FindAllString(text, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		s := strings.TrimSpace(m)
		if len(s) > 0 {
			result = append(result, s)
		}
	}
	// Handle text without ending punctuation
	if len(result) == 0 {
		s := strings.TrimSpace(text)
		if len(s) > 0 {
			result = append(result, s)
		}
	}
	return result
}

// tokenize splits text into lowercase tokens, stripping punctuation.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	re := regexp.MustCompile(`[a-z0-9]+`)
	return re.FindAllString(text, -1)
}

// classifySentence classifies a sentence against context tokens.
func classifySentence(sentence string, contextTokens []string) ClaimAnalysis {
	sentenceLower := strings.ToLower(sentence)
	sentenceTokens := tokenize(sentence)

	if len(sentenceTokens) == 0 {
		return ClaimAnalysis{
			Sentence:       sentence,
			Classification: NotApplicable,
			Confidence:     1.0,
		}
	}

	// Check for questions and greetings first - these are not claims
	if isQuestion(sentenceLower) || isGreeting(sentenceLower) {
		return ClaimAnalysis{
			Sentence:       sentence,
			Classification: NotApplicable,
			Confidence:     0.9,
		}
	}

	// Build context token set
	contextSet := make(map[string]bool, len(contextTokens))
	for _, t := range contextTokens {
		contextSet[t] = true
	}

	// Count overlap
	overlap := 0
	for _, t := range sentenceTokens {
		if contextSet[t] {
			overlap++
		}
	}

	overlapRatio := float64(overlap) / float64(len(sentenceTokens))

	// Check for negation patterns
	hasNegation := containsNegation(sentenceLower)

	// Classification logic
	if overlapRatio >= 0.5 {
		if hasNegation {
			return ClaimAnalysis{
				Sentence:       sentence,
				Classification: Contradictory,
				Confidence:     roundScore(overlapRatio),
			}
		}
		return ClaimAnalysis{
			Sentence:       sentence,
			Classification: Supported,
			Confidence:     roundScore(overlapRatio),
		}
	}

	if overlapRatio >= 0.2 {
		// Low overlap - check for hedging language
		if containsHedging(sentenceLower) {
			return ClaimAnalysis{
				Sentence:       sentence,
				Classification: Disputed,
				Confidence:     roundScore(overlapRatio),
			}
		}
		return ClaimAnalysis{
			Sentence:       sentence,
			Classification: Unsupported,
			Confidence:     roundScore(1.0 - overlapRatio),
		}
	}

	// Very low or no overlap
	return ClaimAnalysis{
		Sentence:       sentence,
		Classification: Unsupported,
		Confidence:     roundScore(1.0 - overlapRatio),
	}
}

// containsNegation checks for negation patterns in text.
func containsNegation(text string) bool {
	negations := []string{
		"not ", "no ", "never ", "neither ", "nor ",
		"don't", "doesn't", "didn't", "won't", "wouldn't",
		"can't", "cannot", "couldn't", "shouldn't",
		"isn't", "aren't", "wasn't", "weren't",
		"nothing ", "nowhere ", "nobody ",
	}
	for _, n := range negations {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

// containsHedging checks for uncertain/hedging language.
func containsHedging(text string) bool {
	hedging := []string{
		"might ", "may ", "could ", "perhaps ", "possibly ",
		"seems ", "appears ", "probably ", "likely ",
		"suggests ", "indicates ", "unclear ", "unknown ",
		"i think ", "i believe ", "it is possible ",
	}
	for _, h := range hedging {
		if strings.Contains(text, h) {
			return true
		}
	}
	return false
}

// isQuestion checks if text is a question.
func isQuestion(text string) bool {
	return strings.HasSuffix(text, "?")
}

// isGreeting checks if text is a greeting or conversational filler.
func isGreeting(text string) bool {
	greetings := []string{"hello", "hi ", "hey", "good morning", "good afternoon", "good evening", "thanks", "thank you"}
	for _, g := range greetings {
		if strings.HasPrefix(text, g) {
			return true
		}
	}
	return false
}

// roundScore rounds to 2 decimal places.
func roundScore(s float64) float64 {
	return float64(int(s*100+0.5)) / 100
}

