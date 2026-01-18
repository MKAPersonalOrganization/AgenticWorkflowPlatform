package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"school-with-ai/foundation/agentsdk/core"
	"strings"
	"unicode"
)

// LoopDetector detects various types of loops in agent execution.
type LoopDetector struct {
	// Tool call loop detection
	toolCallHistory   []toolCallSignature
	toolLoopThreshold int

	// Tool error loop detection (catches repeated failures even with different args)
	toolErrorHistory   []toolErrorRecord
	toolErrorThreshold int

	// Content repetition detection
	contentChunks          []string
	contentChunkSize       int
	contentLoopThreshold   int
	contentMaxDistance     int
	contentRepetitionCount int

	// Pedagogical loop detection (for educational contexts)
	conceptExplanations  map[string]int // concept -> explanation count
	conceptQuizFailures  map[string]int // concept -> quiz failure count
	pedagogicalThreshold int
}

// toolErrorRecord tracks a tool error for error-based loop detection.
type toolErrorRecord struct {
	toolName  string
	wasError  bool
	errorType string // extracted error pattern
}

// toolCallSignature represents a unique tool call for loop detection.
type toolCallSignature struct {
	toolName string
	argsHash string
}

// LoopDetectorConfig contains configuration for loop detection.
type LoopDetectorConfig struct {
	// ToolLoopThreshold is the number of consecutive identical tool calls
	// before triggering a loop detection. Default: 5
	ToolLoopThreshold int

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

	// PedagogicalThreshold is the number of failed explanations before escalation.
	// Default: 3
	PedagogicalThreshold int
}

// DefaultLoopDetectorConfig returns sensible defaults for loop detection.
func DefaultLoopDetectorConfig() LoopDetectorConfig {
	return LoopDetectorConfig{
		ToolLoopThreshold:    5,
		ToolErrorThreshold:   3, // Trigger after 3 consecutive errors from same tool
		ContentChunkSize:     50,
		ContentLoopThreshold: 10,
		ContentMaxDistance:   250, // 5 * 50
		PedagogicalThreshold: 3,
	}
}

// NewLoopDetector creates a new loop detector with the given configuration.
func NewLoopDetector(config LoopDetectorConfig) *LoopDetector {
	if config.ToolLoopThreshold <= 0 {
		config.ToolLoopThreshold = 5
	}
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
	if config.PedagogicalThreshold <= 0 {
		config.PedagogicalThreshold = 3
	}

	return &LoopDetector{
		toolCallHistory:      make([]toolCallSignature, 0),
		toolLoopThreshold:    config.ToolLoopThreshold,
		toolErrorHistory:     make([]toolErrorRecord, 0),
		toolErrorThreshold:   config.ToolErrorThreshold,
		contentChunks:        make([]string, 0),
		contentChunkSize:     config.ContentChunkSize,
		contentLoopThreshold: config.ContentLoopThreshold,
		contentMaxDistance:   config.ContentMaxDistance,
		conceptExplanations:  make(map[string]int),
		conceptQuizFailures:  make(map[string]int),
		pedagogicalThreshold: config.PedagogicalThreshold,
	}
}

// RecordToolCall records a tool call and checks for tool call loops.
// Returns LoopInfo if a loop is detected, nil otherwise.
func (d *LoopDetector) RecordToolCall(tc core.ToolCall) *LoopInfo {
	// Create signature from tool name and arguments hash
	// Arguments is already a JSON string in core.ToolCall
	hash := sha256.Sum256([]byte(tc.Function.Arguments))
	sig := toolCallSignature{
		toolName: tc.Function.Name,
		argsHash: hex.EncodeToString(hash[:8]), // First 8 bytes is enough
	}

	d.toolCallHistory = append(d.toolCallHistory, sig)

	// Count consecutive identical calls from the end
	consecutive := 0
	for i := len(d.toolCallHistory) - 1; i >= 0; i-- {
		if d.toolCallHistory[i] == sig {
			consecutive++
		} else {
			break
		}
	}

	if consecutive >= d.toolLoopThreshold {
		return &LoopInfo{
			Type:        LoopTypeToolCall,
			Description: "Agent is repeatedly calling the same tool with identical arguments",
			Occurrences: consecutive,
			Pattern:     tc.Function.Name,
		}
	}

	return nil
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
		toolName:  toolName,
		wasError:  isError,
		errorType: errorType,
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
		if rec.toolName == toolName && rec.wasError {
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

// RecordQuizFailure records that a student failed a quiz on a concept.
func (d *LoopDetector) RecordQuizFailure(concept string) {
	d.conceptQuizFailures[concept]++
}

// RecordQuizSuccess records that a student passed a quiz on a concept.
// This resets the failure count for that concept.
func (d *LoopDetector) RecordQuizSuccess(concept string) {
	d.conceptQuizFailures[concept] = 0
}

// CheckPedagogicalLoop checks if we're in an unproductive teaching loop.
// Returns LoopInfo if a pedagogical loop is detected, nil otherwise.
func (d *LoopDetector) CheckPedagogicalLoop(concept string) *LoopInfo {
	explanations := d.conceptExplanations[concept]
	failures := d.conceptQuizFailures[concept]

	// If we've explained multiple times and student keeps failing,
	// we might need a different approach
	if explanations >= d.pedagogicalThreshold && failures >= 2 {
		return &LoopInfo{
			Type: LoopTypePedagogical,
			Description: "Student continues to struggle with concept after multiple explanations. " +
				"Consider: different approach, prerequisite review, or human tutor escalation.",
			Occurrences: explanations,
			Pattern:     concept,
		}
	}

	return nil
}

// Reset clears all loop detection state.
func (d *LoopDetector) Reset() {
	d.toolCallHistory = make([]toolCallSignature, 0)
	d.toolErrorHistory = make([]toolErrorRecord, 0)
	d.contentChunks = make([]string, 0)
	d.contentRepetitionCount = 0
	d.conceptExplanations = make(map[string]int)
	d.conceptQuizFailures = make(map[string]int)
}

// GetStats returns statistics about the current detection state.
func (d *LoopDetector) GetStats() map[string]any {
	return map[string]any{
		"tool_calls_recorded":      len(d.toolCallHistory),
		"content_repetition_count": d.contentRepetitionCount,
		"concepts_explained":       len(d.conceptExplanations),
		"concepts_with_failures":   len(d.conceptQuizFailures),
	}
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
