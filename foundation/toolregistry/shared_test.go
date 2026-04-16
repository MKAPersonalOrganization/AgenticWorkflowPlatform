package toolregistry_test

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/toolregistry"
	"context"

	"github.com/a-h/templ"
)

// testPlugin implements toolregistry.PluginProvider for testing purposes.
type testPlugin struct{}

func (p *testPlugin) ID() string   { return "calculator" }
func (p *testPlugin) Icon() string { return "<svg></svg>" }

func (p *testPlugin) DefaultSets() []toolregistry.ToolSet {
	return []toolregistry.ToolSet{
		{
			ID:          "calculator-set",
			Name:        "Calculator",
			Description: "Basic math operations",
			Category:    "Mathematics",
			ToolIDs:     []string{"add", "subtract", "multiply", "divide", "power", "sqrt"},
		},
	}
}

func (p *testPlugin) DefaultTools() []toolregistry.Tool {
	return []toolregistry.Tool{
		{ID: "add", Name: "Add", Description: "Add two numbers", ToolSetID: "calculator-set"},
		{ID: "subtract", Name: "Subtract", Description: "Subtract two numbers", ToolSetID: "calculator-set"},
		{ID: "multiply", Name: "Multiply", Description: "Multiply two numbers", ToolSetID: "calculator-set"},
		{ID: "divide", Name: "Divide", Description: "Divide two numbers", ToolSetID: "calculator-set"},
		{ID: "power", Name: "Power", Description: "Raise to power", ToolSetID: "calculator-set"},
		{ID: "sqrt", Name: "Sqrt", Description: "Square root", ToolSetID: "calculator-set"},
	}
}

func (p *testPlugin) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *testPlugin) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	return nil, nil // Not needed for these tests
}

func (p *testPlugin) GetUITemplate() templ.Component { return nil }
func (p *testPlugin) UIHandledTools() []string       { return nil }
