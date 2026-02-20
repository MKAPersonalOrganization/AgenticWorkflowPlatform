package cognitive

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/toolregistry"
)

// Provider implements the toolregistry.PluginProvider interface for the cognitive toolset.
type Provider struct{}

func (p *Provider) ID() string {
	return "cognitive"
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "cognitive-set",
			Name:        "Cognitive",
			Description: "Tools for agent reasoning and thinking",
			Category:    "Reasoning",
			ToolIDs:     []string{"think"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "think",
			Name:        "Think",
			Description: "Use when you want to think through a problem, clarify assumptions, or break down steps.",
			Category:    "Reasoning",
			ToolSetID:   "cognitive-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "think":
		return &ThinkTool{}, nil
	default:
		return nil, nil // Not handled by this plugin
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"think"}
}

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
