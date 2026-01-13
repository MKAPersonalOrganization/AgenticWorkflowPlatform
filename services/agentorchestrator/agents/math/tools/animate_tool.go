package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrGnuplotScriptRequired     = errors.New("gnuplot_script parameter is required")
	ErrTerminalCommandNotAllowed = errors.New("gnuplot_script should not contain 'set terminal' command - terminal type is managed automatically")
	ErrOutputCommandNotAllowed   = errors.New("gnuplot_script should not contain 'set output' command - output path is managed automatically")
	ErrAnimateTitleTooLong       = errors.New("title exceeds maximum length of 100 characters")
	ErrInvalidAnimationFormat    = errors.New("format must be 'gif' or 'webp'")
	ErrDelayTooLarge             = errors.New("delay exceeds maximum of 500")
	ErrAnimationFileNotCreated   = errors.New("gnuplot did not create the expected output file")
)

// AnimateArgs represents the arguments for the animate tool.
type AnimateArgs struct {
	GnuplotScript string `json:"gnuplot_script"` // Complete gnuplot animation script
	Format        string `json:"format"`         // "gif" or "webp"
	Delay         int    `json:"delay"`          // Delay between frames in hundredths of a second (default: 50 for 20fps)
	Title         string `json:"title"`          // Animation title (for reference/logging)
}

// AnimateTool implements the Tool interface for generating animations with gnuplot.
type AnimateTool struct{}

// NewAnimateTool creates a new animate tool.
func NewAnimateTool() *AnimateTool {
	return &AnimateTool{}
}

func (a *AnimateTool) Name() string {
	return "animate"
}

func (a *AnimateTool) Description() string {
	return "Generate animated mathematical visualizations using gnuplot. Creates animated GIF or WebP files by rendering multiple frames. Perfect for showing function transformations, rotating 3D plots, parameter variations, wave propagation, and dynamic mathematical concepts."
}

func (a *AnimateTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"gnuplot_script": map[string]any{
				"type": "string",
				"description": `Complete gnuplot animation script. REQUIRED. The script must use a loop (do for) to generate multiple frames.

CRITICAL REQUIREMENTS:
1. DO NOT include 'set terminal' - it will be added automatically based on format
2. DO NOT include 'set output' - output path is managed automatically
3. MUST include a loop to generate frames: do for [variable=start:end:step] { ... }
4. Loop variable MUST use INTEGER values only (gnuplot limitation)
5. Each iteration of the loop should plot one frame

GNUPLOT ANIMATION LOOP SYNTAX:
- CORRECT: do for [i=1:100] { plot sin(x + i*0.1) }  ✓ Integer iterator
- CORRECT: do for [angle=0:360:5] { ... }  ✓ Integer values
- WRONG: do for [t=0:10:0.1] { ... }  ✗ Decimal step not allowed
- WORKAROUND: do for [i=0:100] { t = i*0.1; plot sin(x - t) }  ✓ Use integer iterator, calculate decimal inside

KEY POINT: The loop iterator (start:end:step) must be integers. If you need decimal values, use an integer iterator and multiply inside the loop.

EXAMPLE SCRIPTS:

Rotating 3D surface:
unset border
unset tics
set view equal xyz
set pm3d
do for [ang=0:360:5] {
  set view 60, ang
  splot x**2 + y**2 with pm3d
}

Wave propagation (correct way with decimals):
set xrange [0:10]
set yrange [-2:2]
set grid
do for [i=0:100] {
  t = i * 0.2
  plot sin(x - t) with lines title sprintf('t = %.1f', t)
}

Function transformation:
set xrange [-10:10]
set grid
do for [i=1:30] {
  a = i * 0.1
  plot a*sin(x) with lines title sprintf('y = %.1f sin(x)', a)
}

Parametric curve evolution:
set parametric
set trange [0:2*pi]
do for [n=1:10] {
  plot cos(n*t), sin(n*t) with lines title sprintf('n = %d', n)
}`,
			},
			"format": map[string]any{
				"type":        "string",
				"description": "Animation format. 'gif' for animated GIF (widely supported), 'webp' for animated WebP (smaller file size, modern browsers). Defaults to 'gif'.",
				"enum":        []string{"gif", "webp"},
				"default":     "gif",
			},
			"delay": map[string]any{
				"type":        "integer",
				"description": "Delay between frames in hundredths of a second. Default is 50 (0.5 seconds, 20fps). Lower values = faster animation. Examples: 10 (100fps), 50 (20fps), 100 (10fps)",
				"default":     50,
				"minimum":     1,
				"maximum":     500,
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Brief description of the animation for logging/reference purposes. Example: 'Rotating 3D surface', 'Wave propagation'",
				"maxLength":   100,
			},
		},
		"required": []string{"gnuplot_script"},
	}
}

func (a *AnimateTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args AnimateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse animate args: %w", err)
	}

	// Validate required gnuplot_script parameter
	if args.GnuplotScript == "" {
		return "", ErrGnuplotScriptRequired
	}

	// Validate script doesn't contain terminal/output commands
	scriptLower := strings.ToLower(args.GnuplotScript)
	if strings.Contains(scriptLower, "set terminal") || strings.Contains(scriptLower, "set term") {
		return "", ErrTerminalCommandNotAllowed
	}
	if strings.Contains(scriptLower, "set output") {
		return "", ErrOutputCommandNotAllowed
	}

	// Validate format
	if args.Format == "" {
		args.Format = "gif"
	}
	if args.Format != "gif" && args.Format != "webp" {
		return "", fmt.Errorf("%w, got: %s", ErrInvalidAnimationFormat, args.Format)
	}

	// Validate delay
	if args.Delay <= 0 {
		args.Delay = 50 // Default to 20fps
	}
	if args.Delay > 500 {
		return "", fmt.Errorf("%w (got: %d)", ErrDelayTooLarge, args.Delay)
	}

	// Validate title length
	if len(args.Title) > 100 {
		return "", ErrAnimateTitleTooLong
	}

	// Generate the animation
	animationPath, err := executeGnuplotAnimation(args.GnuplotScript, args.Format, args.Delay, args.Title)
	if err != nil {
		return "", fmt.Errorf("failed to generate animation: %w", err)
	}

	// Return result with animation path
	result := map[string]any{
		"success":        true,
		"animation_path": animationPath,
		"animation_url":  "/assets/charts/" + filepath.Base(animationPath),
		"format":         args.Format,
		"delay":          args.Delay,
		"message":        "Animation generated successfully",
	}
	if args.Title != "" {
		result["title"] = args.Title
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal animation result: %w", err)
	}

	return string(resultJSON), nil
}

// executeGnuplotAnimation executes a gnuplot animation script and generates the animation file.
func executeGnuplotAnimation(userScript, format string, delay int, title string) (string, error) {
	// Ensure charts directory exists with secure permissions
	chartsDir := "assets/charts"
	if err := os.MkdirAll(chartsDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create charts directory: %w", err)
	}

	// Generate unique filename using SHA256 (secure hash)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(userScript+title+strconv.FormatInt(time.Now().UnixNano(), 10))))
	hash = hash[:16] // Use first 16 chars
	extension := "." + format
	animationPath := filepath.Join(chartsDir, fmt.Sprintf("animate_%s_%d%s", hash, time.Now().Unix(), extension))

	// Build complete gnuplot script with terminal and output settings
	var script strings.Builder
	script.WriteString("# Auto-generated terminal and output settings\n")

	// Set terminal based on format
	switch format {
	case "gif":
		script.WriteString(fmt.Sprintf("set terminal gif animate delay %d size 800,600\n", delay))
	case "webp":
		script.WriteString(fmt.Sprintf("set terminal webp animate delay %d size 800,600\n", delay))
	}

	script.WriteString(fmt.Sprintf("set output '%s'\n\n", animationPath))
	script.WriteString("# User-provided gnuplot animation script\n")
	script.WriteString(userScript)
	script.WriteString("\n\nunset output\n")

	// Write script to temporary file with secure permissions
	scriptPath := filepath.Join(chartsDir, fmt.Sprintf("animate_script_%s_%d.gp", hash, time.Now().Unix()))
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

	// Verify the animation was created
	if _, err := os.Stat(animationPath); os.IsNotExist(err) {
		return "", fmt.Errorf("%w at %s", ErrAnimationFileNotCreated, animationPath)
	}

	return animationPath, nil
}
