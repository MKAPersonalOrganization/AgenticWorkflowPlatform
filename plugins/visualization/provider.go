package visualization

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/toolregistry"
	"school-with-ai/services/artifact"
)

// Provider implements the toolregistry.PluginProvider interface for the Visualization toolset.
type Provider struct{}

func (p *Provider) ID() string {
	return "visualization"
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "visualization-set",
			Name:        "Visualization",
			Description: "Data visualization and plotting tools",
			Category:    "Math",
			ToolIDs:     []string{"viz-plot", "viz-animate"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "viz-plot", Name: "Plot", Description: "Generate a chart or plot from data", Category: "Visualization", ToolSetID: "visualization-set", CreatedAt: now, UpdatedAt: now},
		{ID: "viz-animate", Name: "Animate", Description: "Create animated visualizations", Category: "Visualization", ToolSetID: "visualization-set", CreatedAt: now, UpdatedAt: now},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	artifactStore, ok := deps.ArtifactStore.(artifact.Service)
	if !ok {
		// Provide fallback or panic if strictly required
	}

	switch toolID {
	case "viz-plot":
		return NewPlotTool(artifactStore), nil
	case "viz-animate":
		return NewAnimateTool(artifactStore), nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"plot", "animate", "viz-plot", "viz-animate"}
}

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
