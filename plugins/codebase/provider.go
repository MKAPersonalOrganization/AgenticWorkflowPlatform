package codebase

import (
	"context"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/toolregistry"
)

// Provider implements the toolregistry.PluginProvider interface for Codebase tools UI.
type Provider struct{}

func (p *Provider) ID() string { return "codebase" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>`)
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
	return []string{"lookup_section", "search_text_in_file"}
}

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
