// Package rubric implements rubric-based evaluation evaluators.
package rubric

import (
	"regexp"
	"strings"
)

// RubricLevel defines a single score level in a rubric.
type RubricLevel struct {
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

// RubricScore is the result of rubric-based evaluation.
type RubricScore struct {
	Score        float64   `json:"score"`
	MatchedLevel int       `json:"matched_level"` // index of best matching level
	LevelScores  []float64 `json:"level_scores"`  // match score per level
	Reasoning    string    `json:"reasoning"`
}

// EvaluateResponse scores a response against rubric levels using heuristic matching.
func EvaluateResponse(response string, levels []RubricLevel) RubricScore {
	if len(levels) == 0 || response == "" {
		return RubricScore{Score: 0, Reasoning: "empty response or levels"}
	}

	responseLower := strings.ToLower(response)
	responseTokens := tokenize(responseLower)
	responseSet := make(map[string]bool)
	for _, t := range responseTokens {
		responseSet[t] = true
	}

	levelScores := make([]float64, len(levels))
	bestLevel := 0
	bestScore := 0.0

	for i, level := range levels {
		levelLower := strings.ToLower(level.Description)
		levelTokens := tokenize(levelLower)

		// Count matching content words (non-stopwords)
		contentMatches := 0
		contentTotal := 0
		for _, t := range levelTokens {
			if isContentWord(t) {
				contentTotal++
				if responseSet[t] {
					contentMatches++
				}
			}
		}

		var score float64
		if contentTotal > 0 {
			score = float64(contentMatches) / float64(contentTotal)
		}
		levelScores[i] = roundScore(score)

		if score > bestScore {
			bestScore = score
			bestLevel = i
		}
	}

	return RubricScore{
		Score:        levels[bestLevel].Score,
		MatchedLevel: bestLevel,
		LevelScores:  levelScores,
		Reasoning:    "best match: level " + string(rune('1'+bestLevel)),
	}
}

// EvaluateToolUse scores tool usage against rubric levels.
func EvaluateToolUse(toolCalls []string, levels []RubricLevel) RubricScore {
	if len(levels) == 0 || len(toolCalls) == 0 {
		return RubricScore{Score: 0, Reasoning: "empty tool calls or levels"}
	}

	// Build a response-like string from tool calls
	response := strings.Join(toolCalls, " ")
	return EvaluateResponse(response, levels)
}

// tokenize splits text into lowercase tokens.
func tokenize(text string) []string {
	re := regexp.MustCompile(`[a-z0-9]+`)
	return re.FindAllString(text, -1)
}

// isContentWord returns true if the word carries semantic content.
func isContentWord(word string) bool {
	if len(word) < 3 {
		return false
	}
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"her": true, "was": true, "one": true, "our": true, "out": true,
		"has": true, "have": true, "been": true, "were": true, "they": true,
		"this": true, "that": true, "with": true, "from": true, "will": true,
		"each": true, "make": true, "like": true, "just": true, "over": true,
		"such": true, "more": true, "than": true, "also": true, "some": true,
		"into": true, "time": true, "very": true, "when": true, "come": true,
		"could": true, "them": true, "then": true, "these": true, "she": true,
	}
	return !stopWords[word]
}

// roundScore rounds to 2 decimal places.
func roundScore(s float64) float64 {
	return float64(int(s*100+0.5)) / 100
}
