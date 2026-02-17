package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	"school-with-ai/services/artifact"
)

var (
	ErrPlotScriptRequired            = errors.New("gnuplot_script parameter is required")
	ErrPlotTitleRequired             = errors.New("title parameter is required")
	ErrPlotTitleTooLong              = errors.New("title exceeds maximum length of 100 characters")
	ErrPlotTerminalCommandNotAllowed = errors.New("gnuplot_script should not contain 'set terminal' command - terminal type is managed automatically")
	ErrPlotOutputCommandNotAllowed   = errors.New("gnuplot_script should not contain 'set output' command - output path is managed automatically")
	ErrPlotFileNotCreated            = errors.New("gnuplot did not create the expected output file")
)

// PlotArgs represents the arguments for the plot tool.
type PlotArgs struct {
	GnuplotScript string `json:"gnuplot_script"` // Complete gnuplot script to execute
	Title         string `json:"title"`          // Plot title (for reference/logging)
}

// PlotTool implements the Tool interface for generating plots with gnuplot.
type PlotTool struct {
	ArtifactStore artifact.Service
}

// NewPlotTool creates a new plot tool.
func NewPlotTool(store artifact.Service) *PlotTool {
	return &PlotTool{
		ArtifactStore: store,
	}
}

func (p *PlotTool) Name() string {
	return "plot"
}

func (p *PlotTool) Description() string {
	return "The ONLY plotting/graphing tool available. Generates plots by executing gnuplot scripts via CLI. You write a complete gnuplot script, the tool executes it through the gnuplot command-line interface, and returns a PNG image. This is a script-based tool - you provide the gnuplot commands, and the CLI tool processes them. Supports all gnuplot features: function plots, data plots, 3D visualizations, multiple plots, custom styling, etc."
}

func (p *PlotTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"gnuplot_script": {
				Type: agent.TypeString,
				Description: `Complete gnuplot script to execute. REQUIRED. The script should be a valid gnuplot program that generates a plot.

CRITICAL REQUIREMENTS:
1. DO NOT include 'set terminal' or 'set output' commands - these are added automatically
2. The tool will automatically configure: set terminal pngcairo size 800,600 enhanced font 'Verdana,12'
3. Output file path is managed automatically

GNUPLOT SYNTAX GUIDE:
- Powers: x**2, x**3 (NOT x^2)
- Multiplication: MUST be explicit (2*x NOT 2x)
- Functions: sin(x), cos(x), tan(x), exp(x), log(x), sqrt(x), abs(x)
- Plot command: plot sin(x) title 'Sine', x**2 with lines
- Multiple plots: plot sin(x), cos(x), tan(x)
- Data plots: Use inline data with special filenames or provide data inline
- Ranges: set xrange [-10:10]; set yrange [-5:5]
- Labels: set xlabel 'X Axis'; set ylabel 'Y Axis'; set title 'My Plot'
- Grid: set grid
- Styles: with lines, with points, with linespoints, with boxes

EXAMPLE SCRIPTS:

Simple function plot:
set title 'Sine Wave'
set xlabel 'x'
set ylabel 'sin(x)'
set grid
set xrange [-6.28:6.28]
plot sin(x) with lines linewidth 2 title 'y = sin(x)'

Multiple functions:
set title 'Trigonometric Functions'
set grid
set xrange [-6.28:6.28]
plot sin(x) title 'sin(x)', cos(x) title 'cos(x)', tan(x) title 'tan(x)'

Polynomial with intercepts:
set title 'Quadratic Function'
set grid
set xrange [-2:6]
plot x**2 - 4*x + 3 with lines title 'y = x² - 4x + 3', 0 with lines dashtype 2 notitle

Data plot with inline data:
set title 'Scatter Plot'
set grid
plot '-' with points pointtype 7 pointsize 1.5 title 'Data Points'
1 2.3
2 4.1
3 5.8
4 7.2
5 9.1
e

3D surface:
set title '3D Surface Plot'
set grid
splot x**2 + y**2 with lines

Remember: The script will be executed as-is, so ensure it's complete and valid gnuplot syntax.`,
			},
			"title": {
				Type:        agent.TypeString,
				Description: "REQUIRED. A descriptive name for the plot that will be used as the image filename. Use lowercase with hyphens, e.g., 'sine-wave', 'quadratic-function', 'velocity-vs-time'. This will be the actual filename.",
			},
		},
		Required: []string{"gnuplot_script", "title"},
	}
}

func (p *PlotTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params PlotArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse plot args: %w", err)), nil
	}

	// Validate required parameters
	if params.GnuplotScript == "" {
		return agent.ErrorResult(ErrPlotScriptRequired), nil
	}
	if params.Title == "" {
		return agent.ErrorResult(ErrPlotTitleRequired), nil
	}

	// Validate script doesn't contain terminal/output commands (we manage these)
	scriptLower := strings.ToLower(params.GnuplotScript)
	if strings.Contains(scriptLower, "set terminal") {
		return agent.ErrorResult(ErrPlotTerminalCommandNotAllowed), nil
	}
	if strings.Contains(scriptLower, "set output") {
		return agent.ErrorResult(ErrPlotOutputCommandNotAllowed), nil
	}

	// Validate title length
	if len(params.Title) > 100 {
		return agent.ErrorResult(ErrPlotTitleTooLong), nil
	}

	// Generate the plot
	imagePath, err := executeGnuplotScript(ctx, params.GnuplotScript, params.Title)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to generate plot: %w", err)), nil
	}
	defer func() { _ = os.Remove(imagePath) }() // Clean up local file after uploading

	// Save to ArtifactStore
	// DEBUG: Print context
	fmt.Printf("[PlotTool] Context - UserID: %s, SessionID: %s\n", args.UserID, args.SessionID)

	if args.UserID == "" || args.SessionID == "" {
		return agent.JSONResult(map[string]any{
			"success":    true,
			"image_path": imagePath,
			"image_url":  "/assets/charts/" + filepath.Base(imagePath),
			"message":    "Plot generated successfully (but not saved to artifact store due to missing context)",
			"title":      params.Title,
		}), nil
	}

	file, err := os.Open(imagePath)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to open generated plot: %w", err)), nil
	}
	defer file.Close()

	filename := filepath.Base(imagePath)
	_, err = p.ArtifactStore.Save(ctx, args.UserID, args.SessionID, filename, file)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to save plot to artifact store: %w", err)), nil
	}

	// Return result as map
	result := map[string]any{
		"success":    true,
		"image_path": imagePath,
		"image_url":  "/v1/threads/" + args.SessionID + "/artifacts/" + filename,
		"message":    "Plot generated successfully and saved as artifact",
		"title":      params.Title,
		"filename":   filename,
	}

	return agent.JSONResult(result), nil
}

// executeGnuplotScript executes a gnuplot script and generates the plot image.
func executeGnuplotScript(ctx context.Context, userScript, title string) (string, error) {
	// Ensure charts directory exists with secure permissions
	chartsDir := "assets/charts"
	if err := os.MkdirAll(chartsDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Sanitize title for use as filename (lowercase, hyphens only, alphanumeric)
	sanitizedTitle := strings.ToLower(title)
	sanitizedTitle = strings.ReplaceAll(sanitizedTitle, " ", "-")
	// Remove any characters that aren't alphanumeric or hyphens
	var cleanTitle strings.Builder
	for _, char := range sanitizedTitle {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			cleanTitle.WriteRune(char)
		}
	}
	filename := cleanTitle.String()
	if filename == "" {
		filename = "plot"
	}

	// Add timestamp to make it unique
	imagePath := filepath.Join(chartsDir, fmt.Sprintf("%s_%d.png", filename, time.Now().Unix()))

	// Build complete gnuplot script with terminal and output settings
	var script strings.Builder
	script.WriteString("# Auto-generated terminal and output settings\n")

	// Determine best font for the current language
	font := "Verdana,12"
	tag := i18n.FromContext(ctx)
	if lang := i18n.GetLanguageByTag(tag); lang != nil && lang.IsRTL() {
		// Use a font that supports Arabic well in Gnuplot (Noto Sans Arabic is a good choice if installed)
		font = "Arial Unicode MS,12"
	}

	script.WriteString(fmt.Sprintf("set terminal pngcairo size 800,600 enhanced font '%s'\n", font))
	script.WriteString(fmt.Sprintf("set output '%s'\n\n", imagePath))
	script.WriteString("# User-provided gnuplot script\n")
	script.WriteString(userScript)
	script.WriteString("\n")

	// Write script to temporary file with secure permissions
	scriptPath := filepath.Join(chartsDir, fmt.Sprintf("script_%s_%d.gp", filename, time.Now().Unix()))
	if err := os.WriteFile(scriptPath, []byte(script.String()), 0o600); err != nil {
		return "", fmt.Errorf("failed to write gnuplot script: %w", err)
	}
	defer func() { _ = os.Remove(scriptPath) }() // Clean up script file

	// Validate script path is safe before executing
	// scriptPath is constructed internally, so should be safe, but verify
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	absChartsDir, err := filepath.Abs(chartsDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute charts dir: %w", err)
	}

	// Ensure script is within the charts directory (prevent path traversal)
	relPath, err := filepath.Rel(absChartsDir, absScriptPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("script path is outside charts directory")
	}

	// Validate gnuplot executable exists in PATH
	gnuplotPath, err := exec.LookPath("gnuplot")
	if err != nil {
		return "", fmt.Errorf("gnuplot not found in PATH: %w", err)
	}

	// Execute gnuplot with validated absolute path
	// gnuplotPath is from LookPath (system executable)
	// absScriptPath is internally generated temp file, validated to be within chartsDir
	cmd := exec.Command(gnuplotPath, absScriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gnuplot execution failed: %w\nScript:\n%s\nOutput: %s", err, script.String(), string(output))
	}

	// Verify the image was created
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("%w at %s", ErrPlotFileNotCreated, imagePath)
	}

	return imagePath, nil
}
