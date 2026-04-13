// Package response implements response matching evaluators for the
// evaluation framework, including ROUGE-1 and LLM-as-judge metrics.
package response

import (
	"math"
	"regexp"
	"strings"
)

// ROUGEScore is the result of ROUGE evaluation.
type ROUGEScore struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
}

// ROUGE1 computes unigram (1-gram) overlap F1 score between candidate and reference.
// Returns precision, recall, and F1 in the range [0, 1].
func ROUGE1(candidate, reference string) ROUGEScore {
	candidateTokens := tokenize(candidate)
	referenceTokens := tokenize(reference)

	if len(referenceTokens) == 0 && len(candidateTokens) == 0 {
		return ROUGEScore{Precision: 1.0, Recall: 1.0, F1: 1.0}
	}
	if len(referenceTokens) == 0 || len(candidateTokens) == 0 {
		return ROUGEScore{}
	}

	// Count unigram frequencies
	candidateCounts := countNgrams(candidateTokens, 1)
	referenceCounts := countNgrams(referenceTokens, 1)

	// Compute overlap
	overlap := 0
	for ngram, count := range candidateCounts {
		if refCount, ok := referenceCounts[ngram]; ok {
			if count < refCount {
				overlap += count
			} else {
				overlap += refCount
			}
		}
	}

	precision := float64(overlap) / float64(len(candidateTokens))
	recall := float64(overlap) / float64(len(referenceTokens))

	var f1 float64
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}

	return ROUGEScore{
		Precision: roundScore4(precision),
		Recall:    roundScore4(recall),
		F1:        roundScore4(f1),
	}
}

// tokenize splits text into lowercase tokens, stripping punctuation.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	// Split on non-alphanumeric characters
	re := regexp.MustCompile(`[a-z0-9]+`)
	tokens := re.FindAllString(text, -1)
	// Filter very short tokens (likely noise)
	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if len(t) > 0 {
			result = append(result, t)
		}
	}
	return result
}

// countNgrams counts n-gram occurrences in a token list.
func countNgrams(tokens []string, n int) map[string]int {
	counts := make(map[string]int)
	for i := 0; i <= len(tokens)-n; i++ {
		ngram := strings.Join(tokens[i:i+n], " ")
		counts[ngram]++
	}
	return counts
}

// roundScore rounds to 4 decimal places.
func roundScore4(s float64) float64 {
	return math.Round(s*10000) / 10000
}
