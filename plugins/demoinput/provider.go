package demoinput

import (
	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"
	"context"
	"time"

	"github.com/a-h/templ"
)

// ToolSetID is the permission key for enabling this plugin.
const ToolSetID = "demoinput-set"

// Provider implements toolregistry.PluginProvider for the demo input plugin.
type Provider struct{}

func (p *Provider) ID() string   { return "demoinput" }
func (p *Provider) Icon() string { return toolregistry.BuildIconSVG(`<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>`) }

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          ToolSetID,
			Name:        "Demo Input",
			Description: "Demo tools that use io.Read() for custom user input",
			Category:    "Demo",
			ToolIDs:     []string{"greeting"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "greeting",
			Name:        "Greeting",
			Description: "Ask the user for their name and greet them (demo of io.Read())",
			Category:    "Demo",
			ToolSetID:   ToolSetID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	if toolID == "greeting" {
		return &GreetingTool{}, nil
	}
	return nil, nil
}

func (p *Provider) GetUITemplate() templ.Component { return nil }
func (p *Provider) UIHandledTools() []string        { return nil }
