package calculator

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/toolregistry"
)

// Provider implements the toolregistry.PluginProvider interface for the Calculator toolset.
type Provider struct{}

func (p *Provider) ID() string {
	return "calculator"
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "calculator-set",
			Name:        "Calculator",
			Description: "Basic mathematical calculation tools",
			Category:    "Math",
			ToolIDs:     []string{"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power", "calc-sqrt", "math-solve"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "calc-add", Name: "Add", Description: "Add two numbers together", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "calc-subtract", Name: "Subtract", Description: "Subtract one number from another", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "calc-multiply", Name: "Multiply", Description: "Multiply two numbers", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "calc-divide", Name: "Divide", Description: "Divide one number by another", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "calc-power", Name: "Power", Description: "Raise a number to a power", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "calc-sqrt", Name: "Square Root", Description: "Calculate the square root of a number", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "math-solve", Name: "Solve Equation", Description: "Solve algebraic equations", Category: "Math", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "calc-add":
		return &AddTool{}, nil
	case "calc-subtract":
		return &SubtractTool{}, nil
	case "calc-multiply":
		return &MultiplyTool{}, nil
	case "calc-divide":
		return &DivideTool{}, nil
	case "calc-sqrt":
		return &SqrtTool{}, nil
	case "calc-power":
		return &PowerTool{}, nil
	case "math-solve":
		return NewEquationSolverTool(), nil
	default:
		return nil, nil // Not handled by this plugin
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"calculate", "add", "subtract", "multiply", "divide", "power", "sqrt", "math-solve"}
}

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
