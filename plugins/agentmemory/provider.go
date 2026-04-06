// Package agentmemory registers the Agent Memory plugin in the tool registry.
// The actual tool (preloadmemorytool) is an ADK-native request processor
// injected directly by the runner when this plugin's tool set is enabled.
// CreateExecutable returns nil because the tool lifecycle is managed by
// ADK-Go, not the foundation ExecutableTool pipeline.
package agentmemory

import (
	"context"
	"time"

	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"

	"github.com/a-h/templ"
)

// ToolSetID is the permission key checked by the runner before injecting the
// ADK-native preloadmemorytool and wiring the memory service. Exported so the
// adapter layer can reference it without a magic string.
const ToolSetID = "memory-set"

// Provider implements toolregistry.PluginProvider for the Agent Memory plugin.
type Provider struct{}

func (p *Provider) ID() string { return "agent-memory" }

func (p *Provider) Icon() string {
	// Lucide "brain" icon paths.
	return toolregistry.BuildIconSVG(`<path d="M12 5a3 3 0 1 0-5.997.125 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z"/><path d="M12 5a3 3 0 1 1 5.997.125 4 4 0 0 1 2.526 5.77 4 4 0 0 1-.556 6.588A4 4 0 1 1 12 18Z"/><path d="M15 13a4.5 4.5 0 0 1-3-4 4.5 4.5 0 0 1-3 4"/><path d="M17.599 6.5a3 3 0 0 0 .399-1.375"/><path d="M6.003 5.125A3 3 0 0 0 6.401 6.5"/><path d="M3.477 10.896a4 4 0 0 1 .585-.396"/><path d="M19.938 10.5a4 4 0 0 1 .585.396"/><path d="M6 18a4 4 0 0 1-1.967-.516"/><path d="M19.967 17.484A4 4 0 0 1 18 18"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{{
		ID:          ToolSetID,
		Name:        "Agent Memory",
		Description: "Enables cross-session memory so agents remember past conversations",
		Category:    "Intelligence",
		ToolIDs:     []string{"preload_memory", "load_memory"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "preload_memory",
			Name:        "Preload Memory",
			Description: "Auto-searches past conversations each turn and injects relevant context into the agent's instructions",
			Category:    "Intelligence",
			ToolSetID:   ToolSetID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "load_memory",
			Name:        "Load Memory",
			Description: "Lets the agent explicitly search past conversations on demand with a custom query",
			Category:    "Intelligence",
			ToolSetID:   ToolSetID,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) SettingsSchema() []toolregistry.SettingsField { return nil }

// CreateExecutable returns nil because preloadmemorytool is an ADK-native
// request processor. The runner injects it directly when this tool set is
// enabled — it cannot go through the foundation ExecutableTool pipeline
// because it needs ADK's ctx.Memory() and ProcessRequest lifecycle.
func (p *Provider) CreateExecutable(_ context.Context, _ string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	return nil, nil
}

func (p *Provider) GetUITemplate() templ.Component { return nil }
func (p *Provider) UIHandledTools() []string        { return nil }
