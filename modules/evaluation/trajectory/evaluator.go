// Package trajectory implements tool trajectory evaluation by comparing
// an agent's actual tool call sequence against expected tool calls.
//
// It supports three match types:
//   - EXACT: Position-by-position match; tool name AND args must match exactly.
//   - IN_ORDER: All expected tools found in actual in correct order, but other
//     tools may be interleaved. Uses longest common subsequence.
//   - ANY_ORDER: All expected tools found in actual regardless of order.
package trajectory

import (
	"encoding/json"
	"math"
)

// MatchType defines how tool calls are compared.
type MatchType string

const (
	MatchExact    MatchType = "EXACT"
	MatchInOrder  MatchType = "IN_ORDER"
	MatchAnyOrder MatchType = "ANY_ORDER"
)

// ExpectedToolCall describes an expected tool invocation.
type ExpectedToolCall struct {
	ToolName  string         `json:"tool_name"`
	Args      map[string]any `json:"args,omitempty"`
	MatchType MatchType      `json:"match_type,omitempty"` // defaults to EXACT
}

// ActualToolCall describes an actual tool invocation made by the agent.
type ActualToolCall struct {
	ToolName string         `json:"tool_name"`
	Args     map[string]any `json:"args"`
}

// TrajectoryScore is the result of comparing expected vs actual trajectories.
type TrajectoryScore struct {
	Score      float64          `json:"score"`
	Matched    int              `json:"matched"`
	Total      int              `json:"total"`
	Missing    []string         `json:"missing,omitempty"`
	Extra      []string         `json:"extra,omitempty"`
	OutOfOrder []string         `json:"out_of_order,omitempty"`
	Details    []ToolCallDetail `json:"details"`
}

// ToolCallDetail provides per-call comparison information.
type ToolCallDetail struct {
	ToolName   string  `json:"tool_name"`
	Expected   bool    `json:"expected"`
	Actual     bool    `json:"actual"`
	Matched    bool    `json:"matched"`
	ParamMatch float64 `json:"param_match"` // 0.0 - 1.0
	Position   int     `json:"position"`
}

// Evaluate compares expected vs actual tool call sequences and returns a score.
func Evaluate(expected []ExpectedToolCall, actual []ActualToolCall) TrajectoryScore {
	if len(expected) == 0 {
		return TrajectoryScore{Score: 1.0, Total: 0, Matched: 0}
	}
	if len(actual) == 0 {
		details := make([]ToolCallDetail, len(expected))
		missing := make([]string, len(expected))
		for i, e := range expected {
			details[i] = ToolCallDetail{
				ToolName:   e.ToolName,
				Expected:   true,
				Actual:     false,
				Matched:    false,
				ParamMatch: 0,
				Position:   i,
			}
			missing[i] = e.ToolName
		}
		return TrajectoryScore{
			Score:   0,
			Total:   len(expected),
			Missing: missing,
			Details: details,
		}
	}

	// Determine match type from first expected call (default to EXACT).
	mt := MatchExact
	if len(expected) > 0 && expected[0].MatchType != "" {
		mt = expected[0].MatchType
	}

	switch mt {
	case MatchExact:
		return evaluateExact(expected, actual)
	case MatchInOrder:
		return evaluateInOrder(expected, actual)
	case MatchAnyOrder:
		return evaluateAnyOrder(expected, actual)
	default:
		return evaluateExact(expected, actual)
	}
}

// evaluateExact does position-by-position matching.
func evaluateExact(expected []ExpectedToolCall, actual []ActualToolCall) TrajectoryScore {
	total := len(expected)
	matched := 0
	details := make([]ToolCallDetail, 0, total)
	missing := []string{}
	extra := []string{}
	outOfOrder := []string{}

	actualUsed := make([]bool, len(actual))

	for i, e := range expected {
		if i < len(actual) {
			a := actual[i]
			paramMatch := compareArgs(e.Args, a.Args)
			isMatch := e.ToolName == a.ToolName && paramMatch == 1.0
			actualUsed[i] = isMatch
			if isMatch {
				matched++
			} else {
				if e.ToolName != a.ToolName {
					outOfOrder = append(outOfOrder, e.ToolName)
				}
			}
			details = append(details, ToolCallDetail{
				ToolName:   e.ToolName,
				Expected:   true,
				Actual:     true,
				Matched:    isMatch,
				ParamMatch: paramMatch,
				Position:   i,
			})
		} else {
			details = append(details, ToolCallDetail{
				ToolName:   e.ToolName,
				Expected:   true,
				Actual:     false,
				Matched:    false,
				ParamMatch: 0,
				Position:   i,
			})
			missing = append(missing, e.ToolName)
		}
	}

	// Find extra tool calls
	for i, a := range actual {
		if !actualUsed[i] {
			extra = append(extra, a.ToolName)
		}
	}

	score := float64(matched) / float64(total)
	return TrajectoryScore{
		Score:      roundScore(score),
		Matched:    matched,
		Total:      total,
		Missing:    nonEmpty(missing),
		Extra:      nonEmpty(extra),
		OutOfOrder: nonEmpty(outOfOrder),
		Details:    details,
	}
}

// evaluateInOrder uses longest common subsequence to find expected tools
// in actual in the correct order, allowing interleaved extras.
func evaluateInOrder(expected []ExpectedToolCall, actual []ActualToolCall) TrajectoryScore {
	total := len(expected)

	// Build LCS: for each expected tool, find it in actual after previous match
	matched := 0
	actualPos := 0
	details := make([]ToolCallDetail, 0, total)
	missing := []string{}
	outOfOrder := []string{}
	actualMatched := make(map[int]bool) // actual indices that matched

	for i, e := range expected {
		found := false
		for j := actualPos; j < len(actual); j++ {
			if actual[j].ToolName == e.ToolName {
				paramMatch := compareArgs(e.Args, actual[j].Args)
				isFullMatch := paramMatch == 1.0
				details = append(details, ToolCallDetail{
					ToolName:   e.ToolName,
					Expected:   true,
					Actual:     true,
					Matched:    isFullMatch,
					ParamMatch: paramMatch,
					Position:   i,
				})
				if isFullMatch {
					matched++
				} else {
					outOfOrder = append(outOfOrder, e.ToolName+" (args differ)")
				}
				actualMatched[j] = true
				actualPos = j + 1
				found = true
				break
			}
		}
		if !found {
			details = append(details, ToolCallDetail{
				ToolName:   e.ToolName,
				Expected:   true,
				Actual:     false,
				Matched:    false,
				ParamMatch: 0,
				Position:   i,
			})
			missing = append(missing, e.ToolName)
		}
	}

	// Find extras
	extra := []string{}
	for j, a := range actual {
		if !actualMatched[j] {
			extra = append(extra, a.ToolName)
		}
	}

	score := float64(matched) / float64(total)
	return TrajectoryScore{
		Score:      roundScore(score),
		Matched:    matched,
		Total:      total,
		Missing:    nonEmpty(missing),
		Extra:      nonEmpty(extra),
		OutOfOrder: nonEmpty(outOfOrder),
		Details:    details,
	}
}

// evaluateAnyOrder checks if all expected tools exist in actual
// regardless of position.
func evaluateAnyOrder(expected []ExpectedToolCall, actual []ActualToolCall) TrajectoryScore {
	total := len(expected)
	matched := 0
	details := make([]ToolCallDetail, 0, total)
	missing := []string{}
	extra := []string{}

	// Count actual tool occurrences
	actualCounts := make(map[string]int)
	actualArgs := make(map[string][]map[string]any)
	for _, a := range actual {
		actualCounts[a.ToolName]++
		actualArgs[a.ToolName] = append(actualArgs[a.ToolName], a.Args)
	}

	// Track which actual slots are consumed
	consumed := make(map[string]int) // tool_name -> count consumed

	for i, e := range expected {
		avail := actualCounts[e.ToolName]
		used := consumed[e.ToolName]
		if used < avail {
			// Find best arg match among available occurrences
			bestParam := 0.0
			args := actualArgs[e.ToolName][used:]
			for _, aArgs := range args {
				pm := compareArgs(e.Args, aArgs)
				if pm > bestParam {
					bestParam = pm
				}
			}
			consumed[e.ToolName] = used + 1
			if bestParam == 1.0 {
				matched++
			}
			details = append(details, ToolCallDetail{
				ToolName:   e.ToolName,
				Expected:   true,
				Actual:     true,
				Matched:    bestParam == 1.0,
				ParamMatch: bestParam,
				Position:   i,
			})
		} else {
			details = append(details, ToolCallDetail{
				ToolName:   e.ToolName,
				Expected:   true,
				Actual:     false,
				Matched:    false,
				ParamMatch: 0,
				Position:   i,
			})
			missing = append(missing, e.ToolName)
		}
	}

	// Find extras: actual tools not consumed
	for toolName, count := range actualCounts {
		if consumedCount := consumed[toolName]; consumedCount < count {
			for i := 0; i < count-consumedCount; i++ {
				extra = append(extra, toolName)
			}
		}
	}

	score := float64(matched) / float64(total)
	return TrajectoryScore{
		Score:   roundScore(score),
		Matched: matched,
		Total:   total,
		Missing: nonEmpty(missing),
		Extra:   nonEmpty(extra),
		Details: details,
	}
}

// compareArgs compares two argument maps and returns a similarity score [0, 1].
func compareArgs(expected, actual map[string]any) float64 {
	// If no expected args, we don't penalize — tool name match is enough.
	if len(expected) == 0 {
		return 1.0
	}
	if len(actual) == 0 {
		return 0.0
	}

	// Exact JSON match → full score
	eJSON, _ := json.Marshal(expected)
	aJSON, _ := json.Marshal(actual)
	if string(eJSON) == string(aJSON) {
		return 1.0
	}

	// Partial: count matching keys
	matches := 0
	total := len(expected)
	for k, ev := range expected {
		if av, ok := actual[k]; ok && jsonEqual(ev, av) {
			matches++
		}
	}
	if total == 0 {
		return 1.0
	}
	return float64(matches) / float64(total)
}

// jsonEqual compares two values by JSON serialization.
func jsonEqual(a, b any) bool {
	aJ, _ := json.Marshal(a)
	bJ, _ := json.Marshal(b)
	return string(aJ) == string(bJ)
}

// roundScore rounds to 2 decimal places.
func roundScore(s float64) float64 {
	return math.Round(s*100) / 100
}

// nonEmpty returns nil if the slice is empty.
func nonEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
