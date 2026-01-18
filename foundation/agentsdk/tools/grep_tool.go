package tools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"school-with-ai/foundation/agentsdk/core"
)

const (
	DefaultTotalMaxMatches = 20000
)

// GrepTool provides the search_text tool powered by ripgrep.
type GrepTool struct {
	// RootDir is the workspace root directory.
	RootDir string
}

// NewGrepTool creates a new GrepTool instance.
func NewGrepTool(rootDir string) *GrepTool {
	return &GrepTool{RootDir: rootDir}
}

// Definition returns the tool definition.
func (t *GrepTool) Definition() []core.Tool {
	return []core.Tool{
		core.NewFunctionTool(
			"search_text_in_file",
			"FAST, optimized search within a SINGLE markdown file powered by `ripgrep`. MUST be used after identifying a specific file via 'browse_library'.",
			map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "The pattern to search for. By default, treated as a Rust-flavored regular expression. Use '\\b' for precise symbol matching (e.g., '\\bMatchMe\\b').",
					},
					"file_path": map[string]any{
						"type":        "string",
						"description": "The specific file path to search within (e.g., 'university/Think-Stats/chapter1.md'). MUST be a file, not a directory. Use 'browse_library' to identify relevant files first.",
					},
					"include": map[string]any{
						"type":        "string",
						"description": "Glob pattern to filter files (e.g., '*.ts', 'src/**'). Recommended for large repositories to reduce noise. Defaults to all files if omitted.",
					},
					"case_sensitive": map[string]any{
						"type":        "boolean",
						"description": "If true, search is case-sensitive. Defaults to false (ignore case) if omitted.",
					},
					"fixed_strings": map[string]any{
						"type":        "boolean",
						"description": "If true, treats the `pattern` as a literal string instead of a regular expression. Defaults to false (basic regex) if omitted.",
					},
					"context": map[string]any{
						"type":        "integer",
						"description": "Show this many lines of context around each match (equivalent to grep -C). Defaults to 0 if omitted.",
					},
					"after": map[string]any{
						"type":        "integer",
						"description": "Show this many lines after each match (equivalent to grep -A). Defaults to 0 if omitted.",
					},
					"before": map[string]any{
						"type":        "integer",
						"description": "Show this many lines before each match (equivalent to grep -B). Defaults to 0 if omitted.",
					},
					"no_ignore": map[string]any{
						"type":        "boolean",
						"description": "If true, searches all files including those usually ignored (like in .gitignore, build/, dist/, etc). Defaults to false if omitted.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of matches to return (for pagination). Defaults to 5 for efficiency.",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of matches to skip (for pagination). Defaults to 0.",
					},
				},
				"required": []string{"pattern"},
			},
			t.execute,
		),
	}
}

type grepArgs struct {
	Pattern       string `json:"pattern"`
	FilePath      string `json:"file_path"`
	Include       string `json:"include"`
	CaseSensitive bool   `json:"case_sensitive"`
	FixedStrings  bool   `json:"fixed_strings"`
	Context       int    `json:"context"`
	After         int    `json:"after"`
	Before        int    `json:"before"`
	NoIgnore      bool   `json:"no_ignore"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

type grepMatch struct {
	FilePath      string
	LineNumber    int
	Line          string
	ParentHeading string // Contextual section heading
}

// Ripgrep JSON output structures
type rgMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgMatchData struct {
	Path       rgPathData  `json:"path"`
	Lines      rgLineData  `json:"lines"`
	LineNumber int         `json:"line_number"`
	AbsoluteOffset int     `json:"absolute_offset"`
	Submatches []rgSubmatch `json:"submatches"`
}

type rgPathData struct {
	Text string `json:"text"`
}

type rgLineData struct {
	Text string `json:"text"`
}

type rgSubmatch struct {
    Match   rgMatchText `json:"match"`
    Start   int         `json:"start"`
    End     int         `json:"end"`
}

type rgMatchText struct {
    Text string `json:"text"`
}


func (t *GrepTool) execute(ctx core.ToolContext, args any) (any, error) {
	var input grepArgs
	if err := core.UnmarshalArgs(args, &input); err != nil {
		return nil, err
	}

	searchPath := input.FilePath
	if searchPath == "" {
		return nil, fmt.Errorf("file_path is required for search_text")
	}
    
    // Resolve absolute path
    absSearchPath, err := filepath.Abs(filepath.Join(t.RootDir, searchPath))
    if err != nil {
        return nil, fmt.Errorf("failed to resolve search path: %w", err)
    }

	// Verify path exists and is a file
	info, err := os.Stat(absSearchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path does not exist: %s", searchPath)
		}
		return nil, fmt.Errorf("failed to access path stats for %s: %w", searchPath, err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path '%s' is a directory. 'search_text' is strictly for single-file search. Use 'browse_library' to find the specific file first.", searchPath)
	}
    // Allow file search too, but if it is a directory check if it is within root logic here if strict security needed.
    // For now assuming the tool is trusted or sandboxed.

	// Build rg command arguments
	cmdArgs := []string{"--json"}

	if !input.CaseSensitive {
		cmdArgs = append(cmdArgs, "--ignore-case")
	}

	if input.FixedStrings {
		cmdArgs = append(cmdArgs, "--fixed-strings")
		cmdArgs = append(cmdArgs, input.Pattern)
	} else {
		cmdArgs = append(cmdArgs, "--regexp", input.Pattern)
	}

	if input.Context > 0 {
		cmdArgs = append(cmdArgs, "--context", strconv.Itoa(input.Context))
	}
	if input.After > 0 {
		cmdArgs = append(cmdArgs, "--after-context", strconv.Itoa(input.After))
	}
	if input.Before > 0 {
		cmdArgs = append(cmdArgs, "--before-context", strconv.Itoa(input.Before))
	}

	if input.NoIgnore {
		cmdArgs = append(cmdArgs, "--no-ignore")
	}

	if input.Include != "" {
		cmdArgs = append(cmdArgs, "--glob", input.Include)
	}
    
    // Add default excludes
    if !input.NoIgnore {
        defaultExcludes := []string{"*.log", "*.tmp", ".git/*", "node_modules/*", "metadata.json"} 
        for _, exclude := range defaultExcludes {
            cmdArgs = append(cmdArgs, "--glob", "!"+exclude)
        }
    }

    cmdArgs = append(cmdArgs, "--threads", "4")
	cmdArgs = append(cmdArgs, absSearchPath)

	// Check if rg is available
	if _, err := exec.LookPath("rg"); err != nil {
		return nil, fmt.Errorf("ripgrep (rg) is not installed or not in PATH")
	}

	cmd := exec.Command("rg", cmdArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err = cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() > 1 {
				return nil, fmt.Errorf("ripgrep failed with code %d", exitErr.ExitCode())
			}
		} else {
            return nil, fmt.Errorf("failed to execute ripgrep: %w", err)
        }
	}

	allMatches := t.parseRipgrepJsonOutput(stdout.Bytes(), filepath.Dir(absSearchPath))

	// Apply pagination
	limit := input.Limit
	if limit <= 0 {
		limit = 5 // Default for efficiency
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	totalMatches := len(allMatches)
	if offset >= totalMatches {
		return fmt.Sprintf("No more matches found (Offset %d exceeds total matches %d).", offset, totalMatches), nil
	}

	end := offset + limit
	if end > totalMatches {
		end = totalMatches
	}
	
	pagedMatches := allMatches[offset:end]

	// Post-process to find parent headings
	for i := range pagedMatches {
		m := &pagedMatches[i]
		m.ParentHeading = t.findParentHeading(absSearchPath, m.LineNumber)
	}

	// Format Output
	searchLocationDescription := fmt.Sprintf("in file \"%s\"", searchPath)
	if len(pagedMatches) == 0 {
		return fmt.Sprintf("No matches found for pattern \"%s\" %s.", input.Pattern, searchLocationDescription), nil
	}

	matchCount := len(pagedMatches)
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Found %d matches (Showing %d-%d of %d) for pattern \"%s\" %s:\n---\n", 
		matchCount, offset+1, end, totalMatches, input.Pattern, searchLocationDescription))

	if end < totalMatches {
		sb.WriteString(fmt.Sprintf("[NOTE: More matches available. Use offset=%d to see more]\n---\n", end))
	}

	// Group by file
	matchesByFile := make(map[string][]grepMatch)
    var files []string
    seenFiles := make(map[string]bool)

	for _, m := range pagedMatches {
		if _, exists := matchesByFile[m.FilePath]; !exists {
            matchesByFile[m.FilePath] = []grepMatch{}
            if !seenFiles[m.FilePath] {
                seenFiles[m.FilePath] = true
                files = append(files, m.FilePath)
            }
        }
		matchesByFile[m.FilePath] = append(matchesByFile[m.FilePath], m)
	}

    sort.Strings(files)

	for _, filePath := range files {
		sb.WriteString(fmt.Sprintf("File: %s\n", filePath))
        matches := matchesByFile[filePath]
        
		for _, m := range matches {
			trimmedLine := strings.TrimSpace(m.Line)
			contextInfo := ""
			if m.ParentHeading != "" {
				contextInfo = fmt.Sprintf(" [Section: %s]", m.ParentHeading)
			}
			sb.WriteString(fmt.Sprintf("L%d:%s %s\n", m.LineNumber, contextInfo, trimmedLine))
		}
		sb.WriteString("---\n")
	}

	return sb.String(), nil
}

// findParentHeading scans the file backwards from lineNumber to find the nearest heading.
func (t *GrepTool) findParentHeading(fullPath string, lineNumber int) string {
	file, err := os.Open(fullPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentLine := 0
	lastHeading := ""
	for scanner.Scan() {
		currentLine++
		if currentLine >= lineNumber {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			lastHeading = line
		}
	}
	return lastHeading
}

func (t *GrepTool) parseRipgrepJsonOutput(output []byte, basePath string) []grepMatch {
	var results []grepMatch
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg rgMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // Skip invalid JSON
		}

		if msg.Type == "match" {
			var data rgMatchData
			if err := json.Unmarshal(msg.Data, &data); err != nil {
				continue
			}

			if data.Path.Text != "" && data.Lines.Text != "" {
                if !filepath.IsAbs(basePath) {
                   // Should typically be absolute, but good to handle if not
                   basePath, _ = filepath.Abs(basePath)
                }
                
                // Construct relative path
                absPath := data.Path.Text
                if !filepath.IsAbs(absPath) {
                    // rg output might be absolute or relative depending on how it was invoked
                    absPath = filepath.Join(basePath, absPath)
                }

                // Correct logic:
                // The provided 'basePath' is the searched directory.  
                // However, we want to report paths relative to the RootDir (Workspace Root) usually, or relative to the search dir? 
                // TS implementation does: filePath: relativeFilePath || path.basename(absoluteFilePath)
                // where relativeFilePath = path.relative(basePath, absoluteFilePath);
                // So it reports relative to the SEARCH directory.
                
                matchAbsPath := data.Path.Text
                if !filepath.IsAbs(matchAbsPath) {
                     matchAbsPath = filepath.Join(basePath, matchAbsPath)
                }
                
                displayPath, err := filepath.Rel(basePath, matchAbsPath)
                if err != nil {
                    displayPath = matchAbsPath
                }

				results = append(results, grepMatch{
					FilePath:   displayPath,
					LineNumber: data.LineNumber,
					Line:       strings.TrimRight(data.Lines.Text, "\r\n"),
				})
			}
		}
	}
	return results
}
