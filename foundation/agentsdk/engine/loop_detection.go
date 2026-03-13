package engine

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode"
)

// LoopDetector detects various types of loops in agent execution.
type LoopDetector struct {
	// Tool error loop detection (catches repeated failures even with different args)
	toolErrorHistory   []toolErrorRecord
	toolErrorThreshold int

	// Content repetition detection
	contentChunks          []string
	contentChunkSize       int
	contentLoopThreshold   int
	contentMaxDistance     int
	contentRepetitionCount int

	// Progress stall detection (repeated failed attempts on the same concept)
	conceptExplanations    map[string]int // concept -> explanation count
	conceptTaskFailures    map[string]int // concept -> task failure count
	progressStallThreshold int
}

// toolErrorRecord tracks a tool error for error-based loop detection.
type toolErrorRecord struct {
	ToolName  string `json:"name"`
	WasError  bool   `json:"err"`
	ErrorType string `json:"type"`
}

// LoopDetectorConfig contains configuration for loop detection.
type LoopDetectorConfig struct {
	// ToolErrorThreshold is the number of consecutive errors from the same tool
	// before triggering a loop detection. This catches cases where the agent
	// tries different arguments but keeps failing. Default: 3
	ToolErrorThreshold int

	// ContentChunkSize is the size of content chunks for repetition detection.
	// Default: 50 characters
	ContentChunkSize int

	// ContentLoopThreshold is the number of repeated chunks before triggering.
	// Default: 10
	ContentLoopThreshold int

	// ContentMaxDistance is the maximum distance between chunks to count as repetition.
	// Default: 5 * ContentChunkSize
	ContentMaxDistance int

	// ProgressStallThreshold is the number of failed attempts before escalation.
	// Default: 3
	ProgressStallThreshold int
}

// DefaultLoopDetectorConfig returns sensible defaults for loop detection.
func DefaultLoopDetectorConfig() LoopDetectorConfig {
	return LoopDetectorConfig{
		ToolErrorThreshold:   3, // Trigger after 3 consecutive errors from same tool
		ContentChunkSize:     50,
		ContentLoopThreshold: 10,
		ContentMaxDistance:   250, // 5 * 50
		ProgressStallThreshold: 3,
	}
}

// NewLoopDetector creates a new loop detector with the given configuration.
func NewLoopDetector(config LoopDetectorConfig) *LoopDetector {
	if config.ToolErrorThreshold <= 0 {
		config.ToolErrorThreshold = 3
	}
	if config.ContentChunkSize <= 0 {
		config.ContentChunkSize = 50
	}
	if config.ContentLoopThreshold <= 0 {
		config.ContentLoopThreshold = 10
	}
	if config.ContentMaxDistance <= 0 {
		config.ContentMaxDistance = config.ContentChunkSize * 5
	}
	if config.ProgressStallThreshold <= 0 {
		config.ProgressStallThreshold = 3
	}

	return &LoopDetector{
		toolErrorHistory:       make([]toolErrorRecord, 0),
		toolErrorThreshold:     config.ToolErrorThreshold,
		contentChunks:          make([]string, 0),
		contentChunkSize:       config.ContentChunkSize,
		contentLoopThreshold:   config.ContentLoopThreshold,
		contentMaxDistance:     config.ContentMaxDistance,
		conceptExplanations:    make(map[string]int),
		conceptTaskFailures:    make(map[string]int),
		progressStallThreshold: config.ProgressStallThreshold,
	}
}

// RecordToolResult records a tool execution result and checks for error-based loops.
// This detects when the same tool keeps failing, even with different arguments.
// Returns LoopInfo if an error loop is detected, nil otherwise.
func (d *LoopDetector) RecordToolResult(toolName string, result string, isError bool) *LoopInfo {
	// Extract error type pattern if this is an error
	errorType := ""
	if isError {
		errorType = extractErrorPattern(result)
	}

	record := toolErrorRecord{
		ToolName:  toolName,
		WasError:  isError,
		ErrorType: errorType,
	}

	d.toolErrorHistory = append(d.toolErrorHistory, record)

	// If this wasn't an error, no loop to detect
	if !isError {
		return nil
	}

	// Count consecutive errors from the same tool
	consecutiveErrors := 0
	for i := len(d.toolErrorHistory) - 1; i >= 0; i-- {
		rec := d.toolErrorHistory[i]
		if rec.ToolName == toolName && rec.WasError {
			consecutiveErrors++
		} else {
			break
		}
	}

	if consecutiveErrors >= d.toolErrorThreshold {
		return &LoopInfo{
			Type: LoopTypeToolError,
			Description: "Tool is repeatedly failing. The agent may be using incorrect syntax " +
				"or the tool may not support the requested operation.",
			Occurrences: consecutiveErrors,
			Pattern:     toolName + ": " + errorType,
		}
	}

	return nil
}

// extractErrorPattern extracts a normalized error pattern from an error message.
// This helps identify similar errors even if the specific details differ.
func extractErrorPattern(errorMsg string) string {
	// Look for common error patterns
	patterns := []string{
		"failed to evaluate expression",
		"mismatched parentheses",
		"expected number",
		"invalid syntax",
		"undefined",
		"not found",
		"timeout",
		"connection",
	}

	lowerMsg := strings.ToLower(errorMsg)
	for _, pattern := range patterns {
		if strings.Contains(lowerMsg, pattern) {
			return pattern
		}
	}

	// If no known pattern, take first 50 chars
	if len(errorMsg) > 50 {
		return errorMsg[:50]
	}
	return errorMsg
}

// RecordContent records content and checks for content repetition loops.
// Returns LoopInfo if a loop is detected, nil otherwise.
func (d *LoopDetector) RecordContent(content string) *LoopInfo {
	// Skip if content is too short
	if len(content) < d.contentChunkSize {
		return nil
	}

	// Clean content: remove code blocks to prevent false positives
	cleanedContent := removeCodeBlocks(content)

	// Split into chunks
	chunks := splitIntoChunks(cleanedContent, d.contentChunkSize)

	// Track chunk positions for repetition detection
	chunkPositions := make(map[string][]int)
	for i, chunk := range chunks {
		// Normalize whitespace
		normalized := normalizeChunk(chunk)
		if len(normalized) < d.contentChunkSize/2 {
			continue // Skip mostly-whitespace chunks
		}
		chunkPositions[normalized] = append(chunkPositions[normalized], i)
	}

	// Check for repeated chunks within close proximity
	var maxRepetitions int
	var repeatedPattern string
	for pattern, positions := range chunkPositions {
		if len(positions) < 2 {
			continue
		}

		// Count repetitions within max distance
		closeRepetitions := 0
		for i := 1; i < len(positions); i++ {
			distance := (positions[i] - positions[i-1]) * d.contentChunkSize
			if distance <= d.contentMaxDistance {
				closeRepetitions++
			}
		}

		if closeRepetitions > maxRepetitions {
			maxRepetitions = closeRepetitions
			repeatedPattern = pattern
		}
	}

	d.contentRepetitionCount = maxRepetitions

	if maxRepetitions >= d.contentLoopThreshold {
		return &LoopInfo{
			Type:        LoopTypeContent,
			Description: "Agent is repeating the same content patterns",
			Occurrences: maxRepetitions,
			Pattern:     truncateString(repeatedPattern, 100),
		}
	}

	return nil
}

// RecordConceptExplanation records that a concept was explained.
func (d *LoopDetector) RecordConceptExplanation(concept string) {
	d.conceptExplanations[concept]++
}

// RecordTaskFailure records that a user failed a task on a concept.
func (d *LoopDetector) RecordTaskFailure(concept string) {
	d.conceptTaskFailures[concept]++
}

// RecordTaskSuccess records that a user passed a task on a concept.
// This resets the failure count for that concept.
func (d *LoopDetector) RecordTaskSuccess(concept string) {
	d.conceptTaskFailures[concept] = 0
}

// CheckProgressStall checks if the user is stuck in an unproductive loop.
// Returns LoopInfo if a progress stall is detected, nil otherwise.
func (d *LoopDetector) CheckProgressStall(concept string) *LoopInfo {
	explanations := d.conceptExplanations[concept]
	failures := d.conceptTaskFailures[concept]

	// If we've attempted multiple times and user keeps failing,
	// we might need a different approach
	if explanations >= d.progressStallThreshold && failures >= 2 {
		return &LoopInfo{
			Type: LoopTypeProgressStall,
			Description: "User has not made progress after multiple attempts. " +
				"Consider: different approach, prerequisite review, or human escalation.",
			Occurrences: explanations,
			Pattern:     concept,
		}
	}

	return nil
}

// Reset clears all loop detection state.
func (d *LoopDetector) Reset() {
	d.toolErrorHistory = make([]toolErrorRecord, 0)
	d.contentChunks = make([]string, 0)
	d.contentRepetitionCount = 0
	d.conceptExplanations = make(map[string]int)
	d.conceptTaskFailures = make(map[string]int)
}

// GetStats returns statistics about the current detection state.
func (d *LoopDetector) GetStats() map[string]any {
	return map[string]any{
		"content_repetition_count": d.contentRepetitionCount,
		"concepts_explained":       len(d.conceptExplanations),
		"concepts_with_failures":   len(d.conceptTaskFailures),
	}
}

// Serialize returns a hex-encoded JSON representation of the detector state.
func (d *LoopDetector) Serialize() string {
	state := loopDetectorState{
		ToolErrorHistory:    d.toolErrorHistory,
		ContentChunks:       d.contentChunks,
		ConceptExplanations: d.conceptExplanations,
		ConceptTaskFailures: d.conceptTaskFailures,
	}
	b, _ := json.Marshal(state)
	return hex.EncodeToString(b)
}

// Load restores the detector state from a hex-encoded JSON string.
func (d *LoopDetector) Load(stateHex string) error {
	if stateHex == "" {
		return nil
	}
	b, err := hex.DecodeString(stateHex)
	if err != nil {
		return err
	}
	var state loopDetectorState
	if err := json.Unmarshal(b, &state); err != nil {
		return err
	}
	d.toolErrorHistory = state.ToolErrorHistory
	d.contentChunks = state.ContentChunks
	d.conceptExplanations = state.ConceptExplanations
	d.conceptTaskFailures = state.ConceptTaskFailures
	return nil
}

type loopDetectorState struct {
	ToolErrorHistory    []toolErrorRecord `json:"toolErrorHistory"`
	ContentChunks       []string          `json:"contentChunks"`
	ConceptExplanations map[string]int    `json:"conceptExplanations"`
	ConceptTaskFailures map[string]int    `json:"conceptTaskFailures"`
}

// Helper functions

// removeCodeBlocks removes markdown code blocks from content.
func removeCodeBlocks(content string) string {
	var result strings.Builder
	inCodeBlock := false
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if !inCodeBlock {
			result.WriteString(line)
			result.WriteString(" ")
		}
	}

	return result.String()
}

// splitIntoChunks splits content into chunks of the specified size.
func splitIntoChunks(content string, chunkSize int) []string {
	var chunks []string
	runes := []rune(content)

	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}

	return chunks
}

// normalizeChunk normalizes a chunk for comparison.
func normalizeChunk(chunk string) string {
	// Collapse whitespace
	var result strings.Builder
	lastWasSpace := false

	for _, r := range chunk {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				result.WriteRune(' ')
				lastWasSpace = true
			}
		} else {
			result.WriteRune(unicode.ToLower(r))
			lastWasSpace = false
		}
	}

	return strings.TrimSpace(result.String())
}

// truncateString truncates a string to the specified length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
