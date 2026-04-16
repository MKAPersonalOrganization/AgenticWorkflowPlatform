package library

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/toolregistry"
	"context"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the Library UI.
type Provider struct{}

func (p *Provider) ID() string { return "library" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<path d="m16 6 4 14"/><path d="M12 6v14"/><path d="M8 8v12"/><path d="M4 4v16"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet { return nil }
func (p *Provider) DefaultTools() []toolregistry.Tool   { return nil }

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	return nil, nil // Not handled by this plugin
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"browse_library"}
}
