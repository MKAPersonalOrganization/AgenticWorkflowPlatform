package todos

import (
	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"
	"context"
	"time"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the
// todos/progress tracking toolset.
// Inspired by Gemini CLI's write_todos tool — enables agents to decompose
// complex requests into visible subtasks and track progress in real-time.
type Provider struct{}

func (p *Provider) ID() string { return "todos" }

func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(
		`<path d="M21 12.5V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h7.5"/>` +
			`<path d="m9 11 3 3L22 4"/>` +
			`<path d="M16 19h6"/>` +
			`<path d="M19 16v6"/>`,
	)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "todos-set",
			Name:        "Progress Tracker",
			Description: "Tools for agent task planning and progress tracking",
			Category:    "Planning",
			ToolIDs:     []string{"write_todos"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "write_todos",
			Name:        "Write Todos",
			Description: "Create and update a structured task list to track progress on multi-step requests. The agent uses this to decompose complex work into visible subtasks with real-time status updates.",
			Category:    "Planning",
			ToolSetID:   "todos-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "write_todos":
		return &WriteTodosTool{}, nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"write_todos"}
}
