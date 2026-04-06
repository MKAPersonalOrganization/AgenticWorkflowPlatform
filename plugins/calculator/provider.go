package calculator

import (
	"context"
	"time"

	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the Calculator toolset.
type Provider struct{}

func (p *Provider) ID() string { return "calculator" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<rect width="16" height="20" x="4" y="2" rx="2"/><line x1="8" x2="16" y1="6" y2="6"/><line x1="16" x2="16" y1="14" y2="18"/><path d="M16 10h.01"/><path d="M12 10h.01"/><path d="M8 10h.01"/><path d="M12 14h.01"/><path d="M8 14h.01"/><path d="M12 18h.01"/><path d="M8 18h.01"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "calculator-set",
			Name:        "Calculator",
			Description: "Basic mathematical calculation tools",
			Category:    "Math",
			ToolIDs:     []string{"add", "subtract", "multiply", "divide", "power", "sqrt", "solve_equation"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "add", Name: "Add", Description: "Add two numbers together", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "subtract", Name: "Subtract", Description: "Subtract one number from another", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "multiply", Name: "Multiply", Description: "Multiply two numbers", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "divide", Name: "Divide", Description: "Divide one number by another", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "power", Name: "Power", Description: "Raise a number to a power", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "sqrt", Name: "Square Root", Description: "Calculate the square root of a number", Category: "Calculator", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
		{ID: "solve_equation", Name: "Solve Equation", Description: "Solve algebraic equations", Category: "Math", ToolSetID: "calculator-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "add":
		return &AddTool{}, nil
	case "subtract":
		return &SubtractTool{}, nil
	case "multiply":
		return &MultiplyTool{}, nil
	case "divide":
		return &DivideTool{}, nil
	case "sqrt":
		return &SqrtTool{}, nil
	case "power":
		return &PowerTool{}, nil
	case "solve_equation":
		return NewEquationSolverTool(), nil
	default:
		return nil, nil // Not handled by this plugin
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"add", "subtract", "multiply", "divide", "power", "sqrt", "solve_equation"}
}
