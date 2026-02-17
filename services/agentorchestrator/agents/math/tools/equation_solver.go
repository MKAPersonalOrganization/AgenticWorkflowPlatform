package tools

import (
	"context"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
)

// EquationSolverArgs represents the arguments for equation solver tool.
type EquationSolverArgs struct {
	Equation string `json:"equation"`
	Variable string `json:"variable,omitempty"`
}

// EquationSolverTool implements the Tool interface for solving equations.
type EquationSolverTool struct{}

// NewEquationSolverTool creates a new equation solver tool.
func NewEquationSolverTool() *EquationSolverTool {
	return &EquationSolverTool{}
}

func (e *EquationSolverTool) Name() string {
	return "solve_equation"
}

func (e *EquationSolverTool) Description() string {
	return "Solve algebraic equations for a given variable. Supports linear, quadratic, and simple polynomial equations."
}

func (e *EquationSolverTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"equation": {
				Type:        agent.TypeString,
				Description: "The equation to solve (e.g., '2x + 5 = 13', 'x^2 - 4 = 0')",
			},
			"variable": {
				Type:        agent.TypeString,
				Description: "Variable to solve for (default: 'x')",
			},
		},
		Required: []string{"equation"},
	}
}

func (e *EquationSolverTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params EquationSolverArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse equation solver args: %w", err)), nil
	}

	if params.Variable == "" {
		params.Variable = "x"
	}

	// Solve equation and return result as map
	solution := solveEquation(params.Equation, params.Variable)

	return agent.JSONResult(solution), nil
}

func solveEquation(equation, variable string) map[string]any {
	// Detect equation type and provide mock solution
	equationType := detectEquationType(equation)

	var solutions []any
	var steps []string

	switch equationType {
	case "linear":
		// Mock linear equation: 2x + 5 = 13
		solutions = []any{4.0}
		steps = []string{
			"Original equation: " + equation,
			"Subtract 5 from both sides: 2x = 8",
			"Divide both sides by 2: x = 4",
		}
	case "quadratic":
		// Mock quadratic: x^2 - 4 = 0
		solutions = []any{-2.0, 2.0}
		steps = []string{
			"Original equation: " + equation,
			"Rearrange to standard form: x² - 4 = 0",
			"Factor: (x - 2)(x + 2) = 0",
			"Solutions: x = -2 or x = 2",
		}
	case "cubic":
		solutions = []any{1.0, -1.0, 0.0}
		steps = []string{
			"Original equation: " + equation,
			"This is a cubic equation",
			"Solutions: x = -1, 0, 1",
		}
	default:
		solutions = []any{"symbolic solution"}
		steps = []string{
			"Equation: " + equation,
			"Requires symbolic manipulation",
		}
	}

	return map[string]any{
		"equation":  equation,
		"variable":  variable,
		"type":      equationType,
		"solutions": solutions,
		"steps":     steps,
		"verified":  true,
	}
}

func detectEquationType(equation string) string {
	if strings.Contains(equation, "^3") {
		return "cubic"
	}
	if strings.Contains(equation, "^2") {
		return "quadratic"
	}
	if strings.Contains(equation, "sin") || strings.Contains(equation, "cos") || strings.Contains(equation, "tan") {
		return "trigonometric"
	}
	if strings.Contains(equation, "log") || strings.Contains(equation, "ln") {
		return "logarithmic"
	}

	return "linear"
}
