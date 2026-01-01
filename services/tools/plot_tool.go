package tools

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PlotArgs represents the arguments for the plot tool
type PlotArgs struct {
	GnuplotScript string `json:"gnuplot_script"` // Complete gnuplot script to execute
	Title         string `json:"title"`          // Plot title (for reference/logging)
}

// PlotTool implements the Tool interface for generating plots with gnuplot
type PlotTool struct{}

// NewPlotTool creates a new plot tool
func NewPlotTool() *PlotTool {
	return &PlotTool{}
}

func (p *PlotTool) Name() string {
	return "plot"
}

func (p *PlotTool) Description() string {
	return "Execute gnuplot scripts to generate high-quality mathematical plots and graphs. You provide a complete gnuplot script, and the tool executes it to create a PNG image. Supports all gnuplot features: function plots, data plots, 3D visualizations, multiple plots, custom styling, etc."
}

func (p *PlotTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"gnuplot_script": map[string]any{
				"type": "string",
				"description": `Complete gnuplot script to execute. REQUIRED. The script should be a valid gnuplot program that generates a plot.

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
			"title": map[string]any{
				"type":        "string",
				"description": "Brief description of the plot for logging/reference purposes. Example: 'Sine wave visualization', 'Quadratic function with roots'",
				"maxLength":   100,
			},
		},
		"required": []string{"gnuplot_script"},
	}
}

func (p *PlotTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args PlotArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse plot args: %w", err)
	}

	// Validate required gnuplot_script parameter
	if args.GnuplotScript == "" {
		return "", fmt.Errorf("gnuplot_script parameter is required")
	}

	// Validate script doesn't contain terminal/output commands (we manage these)
	scriptLower := strings.ToLower(args.GnuplotScript)
	if strings.Contains(scriptLower, "set terminal") {
		return "", fmt.Errorf("gnuplot_script should not contain 'set terminal' command - terminal type is managed automatically")
	}
	if strings.Contains(scriptLower, "set output") {
		return "", fmt.Errorf("gnuplot_script should not contain 'set output' command - output path is managed automatically")
	}

	// Validate title length
	if len(args.Title) > 100 {
		return "", fmt.Errorf("title exceeds maximum length of 100 characters")
	}

	// Generate the plot
	imagePath, err := executeGnuplotScript(args.GnuplotScript, args.Title)
	if err != nil {
		return "", fmt.Errorf("failed to generate plot: %w", err)
	}

	// Return result with image path
	result := map[string]any{
		"success":    true,
		"image_path": imagePath,
		"image_url":  "/assets/charts/" + filepath.Base(imagePath),
		"message":    "Plot generated successfully",
	}
	if args.Title != "" {
		result["title"] = args.Title
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal plot result: %w", err)
	}

	return string(resultJSON), nil
}

// executeGnuplotScript executes a gnuplot script and generates the plot image
func executeGnuplotScript(userScript, title string) (string, error) {
	// Ensure charts directory exists
	chartsDir := "assets/charts"
	if err := os.MkdirAll(chartsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Generate unique filename
	hash := fmt.Sprintf("%x", md5.Sum([]byte(userScript+title+fmt.Sprintf("%d", time.Now().UnixNano()))))
	hash = hash[:16] // Use first 16 chars
	imagePath := filepath.Join(chartsDir, fmt.Sprintf("plot_%s_%d.png", hash, time.Now().Unix()))

	// Build complete gnuplot script with terminal and output settings
	var script strings.Builder
	script.WriteString("# Auto-generated terminal and output settings\n")
	script.WriteString("set terminal pngcairo size 800,600 enhanced font 'Verdana,12'\n")
	script.WriteString(fmt.Sprintf("set output '%s'\n\n", imagePath))
	script.WriteString("# User-provided gnuplot script\n")
	script.WriteString(userScript)
	script.WriteString("\n")

	// Write script to temporary file
	scriptPath := filepath.Join(chartsDir, fmt.Sprintf("script_%s_%d.gp", hash, time.Now().Unix()))
	if err := os.WriteFile(scriptPath, []byte(script.String()), 0o644); err != nil {
		return "", fmt.Errorf("failed to write gnuplot script: %w", err)
	}
	defer func() { _ = os.Remove(scriptPath) }() // Clean up script file

	// Execute gnuplot
	cmd := exec.Command("gnuplot", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gnuplot execution failed: %w\nScript:\n%s\nOutput: %s", err, script.String(), string(output))
	}

	// Verify the image was created
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return "", fmt.Errorf("gnuplot did not create the expected output file at %s", imagePath)
	}

	return imagePath, nil
}
