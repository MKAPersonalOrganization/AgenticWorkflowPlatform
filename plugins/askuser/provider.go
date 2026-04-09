package askuser

import (
	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"
	"context"
	"time"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the
// ask_user tool — a Gemini-style multi-question input tool that lets
// agents request structured user input (choices, text, yes/no).
type Provider struct{}

func (p *Provider) ID() string { return "askuser" }

func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(
		`<circle cx="12" cy="12" r="10"/>` +
			`<path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/>` +
			`<path d="M12 17h.01"/>`,
	)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "askuser-set",
			Name:        "User Input",
			Description: "Tools for requesting structured input from the user",
			Category:    "Interaction",
			ToolIDs:     []string{"ask_user"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "ask_user",
			Name:        "Ask User",
			Description: "Request structured input from the user. Supports choice, text, and yes/no question types with rich options.",
			Category:    "Interaction",
			ToolSetID:   "askuser-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "ask_user":
		return &AskUserTool{}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"ask_user"}
}
