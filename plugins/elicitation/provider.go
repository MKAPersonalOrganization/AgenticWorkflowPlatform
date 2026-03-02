package elicitation

import (
	"context"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/toolregistry"
)

// Provider implements the toolregistry.PluginProvider interface for Elicitation UIs.
type Provider struct{}

func (p *Provider) ID() string { return "elicitation" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet { return nil }
func (p *Provider) DefaultTools() []toolregistry.Tool   { return nil }

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	return nil, nil // Not handled by this plugin
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return nil
}

func (p *Provider) UIHandledElicitations() []string {
	return []string{"choose_multiple", "multiple_choice", "true_false", "short_answer"}
}
