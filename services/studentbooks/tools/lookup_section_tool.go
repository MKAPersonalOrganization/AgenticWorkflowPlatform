package tools

import (
	"bufio"
	"fmt"
	"strings"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/studentbooks"
)

// LookupSectionTool allows searching for specific sections within a library resource.
type LookupSectionTool struct {
	books studentbooks.Service
}

// NewLookupSectionTool creates a new instance of the LookupSectionTool.
func NewLookupSectionTool(books studentbooks.Service) *LookupSectionTool {
	return &LookupSectionTool{
		books: books,
	}
}

// Name returns the tool's name.
func (t *LookupSectionTool) Name() string {
	return "lookup_section"
}

// Description returns a description of the tool.
func (t *LookupSectionTool) Description() string {
	return "Reads a specific section of a library book or chapter. Useful for reading context around a search result."
}

// Parameters returns the schema for the tool's arguments.
func (t *LookupSectionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file (e.g., 'grade-10/math/algebra.md').",
			},
			"section_heading": map[string]any{
				"type":        "string",
				"description": "The exact heading text of the section to read (e.g., 'Quadratic Equations').",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Start reading from this line number (1-based). Overrides section_heading if provided.",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Stop reading at this line number (1-based).",
			},
		},
		"required": []string{"path"},
	}
}

// LookupSectionArgs defines the expected arguments.
type LookupSectionArgs struct {
	Path           string `json:"path"`
	SectionHeading string `json:"section_heading,omitempty"`
	StartLine      int    `json:"start_line,omitempty"`
	EndLine        int    `json:"end_line,omitempty"`
}

// Invoke executes the tool logic.
func (t *LookupSectionTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params LookupSectionArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse args: %w", err)
	}

	content, err := t.books.GetChapterContent(params.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file '%s': %w", params.Path, err)
	}

	// Mode 1: Line number range
	if params.StartLine > 0 {
		return t.extractByLines(content, params.StartLine, params.EndLine)
	}

	// Mode 2: Section Heading
	if params.SectionHeading != "" {
		return t.extractByHeading(content, params.SectionHeading)
	}

	return nil, fmt.Errorf("must provide either 'section_heading' or 'start_line'")
}

func (t *LookupSectionTool) extractByLines(content string, start, end int) (string, error) {
	lines := strings.Split(content, "\n")
	if start < 1 || start > len(lines) {
		return "", fmt.Errorf("start_line %d is out of bounds (file has %d lines)", start, len(lines))
	}

	if end <= 0 || end > len(lines) {
		end = start + 50 // Default window if end not specified
		if end > len(lines) {
			end = len(lines)
		}
	}

	// Adjust for 0-based indexing
	selectedLines := lines[start-1 : end]
	return strings.Join(selectedLines, "\n"), nil
}

func (t *LookupSectionTool) extractByHeading(content, heading string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var sb strings.Builder
	inSection := false
	sectionLevel := 0

	normalizedTarget := strings.TrimSpace(strings.ToLower(heading))

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			// Calculate heading level
			level := 0
			for i := 0; i < len(trimmed) && trimmed[i] == '#'; i++ {
				level++
			}
			title := strings.TrimSpace(trimmed[level:])

			// If we are already in the target section, a new heading of same or higher level ends it
			if inSection {
				if level <= sectionLevel {
					break
				}
			}

			// Check if this is the start of our section
			if !inSection && strings.EqualFold(title, normalizedTarget) {
				inSection = true
				sectionLevel = level
				sb.WriteString(line + "\n") // Include the heading itself
				continue
			}
		}

		if inSection {
			sb.WriteString(line + "\n")
		}
	}

	if sb.Len() == 0 {
		return "", fmt.Errorf("section '%s' not found", heading)
	}

	return sb.String(), nil
}
