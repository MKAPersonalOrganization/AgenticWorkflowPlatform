// Package artifacts registers the Artifact Management plugin in the tool
// registry. The actual tool (loadartifactstool) is an ADK-native request
// processor injected directly by the runner when this plugin's tool set is
// enabled. CreateExecutable returns nil because the tool lifecycle is managed
// by ADK-Go, not the foundation ExecutableTool pipeline.
package artifacts

import (
	"context"
	"time"

	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"

	"github.com/a-h/templ"
)

// ToolSetID is the permission key checked by the runner before injecting
// the ADK-native loadartifactstool. Exported so the adapter layer can
// reference it without a magic string.
const ToolSetID = "artifacts-set"

// Provider implements toolregistry.PluginProvider for the Artifacts plugin.
type Provider struct{}

func (p *Provider) ID() string { return "artifacts" }

func (p *Provider) Icon() string {
	// Lucide "file-stack" icon paths.
	return toolregistry.BuildIconSVG(`<path d="M21 7h-3a2 2 0 0 1-2-2V2"/><path d="M21 6v6.5c0 .8-.7 1.5-1.5 1.5h-7c-.8 0-1.5-.7-1.5-1.5v-9c0-.8.7-1.5 1.5-1.5H17Z"/><path d="M7 8v8.8c0 .3.2.6.4.8.2.2.5.4.8.4H15"/><path d="M3 12v8.8c0 .3.2.6.4.8.2.2.5.4.8.4H11"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{{
		ID:          ToolSetID,
		Name:        "Artifact Management",
		Description: "Lets agents create, version, and load files (reports, CSVs, images) that persist across sessions",
		Category:    "Files",
		ToolIDs:     []string{"load_artifacts"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{{
		ID:          "load_artifacts",
		Name:        "Load Artifacts",
		Description: "Lists available artifacts and loads their content when requested by the agent",
		Category:    "Files",
		ToolSetID:   ToolSetID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

// CreateExecutable returns nil because loadartifactstool is an ADK-native
// request processor. The runner injects it directly when this tool set is
// enabled — it cannot go through the foundation ExecutableTool pipeline
// because it needs ADK's ctx.Artifacts() and ProcessRequest lifecycle.
func (p *Provider) CreateExecutable(_ context.Context, _ string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	return nil, nil
}

func (p *Provider) GetUITemplate() templ.Component { return nil }
func (p *Provider) UIHandledTools() []string        { return nil }
