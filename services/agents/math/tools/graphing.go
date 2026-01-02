package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// GraphingArgs represents the arguments for graphing tool
type GraphingArgs struct {
	Function string  `json:"function"`
	XMin     float64 `json:"x_min"`
	XMax     float64 `json:"x_max"`
}

// GraphingTool implements the Tool interface for plotting mathematical functions
type GraphingTool struct{}

// NewGraphingTool creates a new graphing tool
func NewGraphingTool() *GraphingTool {
	return &GraphingTool{}
}

func (g *GraphingTool) Name() string {
	return "plot_function"
}

func (g *GraphingTool) Description() string {
	return "Generate a text-based visualization or description of a mathematical function's graph. Useful for understanding function behavior, finding intercepts, and identifying key features."
}

func (g *GraphingTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"function": map[string]any{
				"type":        "string",
				"description": "Mathematical function to plot (e.g., 'x^2', 'sin(x)', '2x + 3')",
			},
			"x_min": map[string]any{
				"type":        "number",
				"description": "Minimum x value for the plot",
			},
			"x_max": map[string]any{
				"type":        "number",
				"description": "Maximum x value for the plot",
			},
		},
		"required": []string{"function"},
	}
}

func (g *GraphingTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args GraphingArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse graphing args: %w", err)
	}

	// Set defaults
	if args.XMin == 0 && args.XMax == 0 {
		args.XMin = -10
		args.XMax = 10
	}

	// Mock response with function analysis
	result := map[string]any{
		"function": args.Function,
		"domain":   fmt.Sprintf("[%.1f, %.1f]", args.XMin, args.XMax),
		"features": map[string]any{
			"type":        analyzeFunction(args.Function),
			"y_intercept": "Crosses y-axis at (0, c)",
			"behavior":    describeBehavior(args.Function),
			"key_points":  generateKeyPoints(args.Function, args.XMin, args.XMax),
		},
		"visualization": generateASCIIPlot(args.Function),
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal graphing result: %w", err)
	}

	return string(resultJSON), nil
}

func analyzeFunction(fn string) string {
	// Simple pattern matching for common function types
	switch {
	case contains(fn, "sin") || contains(fn, "cos"):
		return "trigonometric"
	case contains(fn, "^2"):
		return "quadratic/parabola"
	case contains(fn, "^3"):
		return "cubic"
	case contains(fn, "sqrt"):
		return "square root"
	case contains(fn, "log") || contains(fn, "ln"):
		return "logarithmic"
	case contains(fn, "e^") || contains(fn, "exp"):
		return "exponential"
	default:
		return "linear or polynomial"
	}
}

func describeBehavior(fn string) string {
	switch {
	case contains(fn, "sin") || contains(fn, "cos"):
		return "Periodic oscillation between -1 and 1"
	case contains(fn, "^2"):
		return "U-shaped parabola, opens upward"
	case contains(fn, "-x^2"):
		return "Inverted parabola, opens downward"
	case contains(fn, "^3"):
		return "S-shaped curve, increases from left to right"
	case contains(fn, "e^"):
		return "Rapid exponential growth"
	default:
		return "Continuous function with steady change"
	}
}

func generateKeyPoints(fn string, xMin, xMax float64) []map[string]float64 {
	// Generate sample points
	points := []map[string]float64{}
	step := (xMax - xMin) / 5

	for i := 0; i < 6; i++ {
		x := xMin + float64(i)*step
		// Mock y value based on function type
		y := mockEvaluate(fn, x)
		points = append(points, map[string]float64{"x": x, "y": y})
	}

	return points
}

func mockEvaluate(fn string, x float64) float64 {
	// Simple mock evaluation
	switch {
	case contains(fn, "^2"):
		return x * x
	case contains(fn, "^3"):
		return x * x * x
	case contains(fn, "sin"):
		return 0 // Simplified
	default:
		return x
	}
}

func generateASCIIPlot(fn string) string {
	return "  |\n  |    *\n  | *\n  *\n__|_________\n  |         x\n(ASCII representation of " + fn + ")"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
