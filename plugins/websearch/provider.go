package websearch

import (
	"context"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the
// web search toolset.
// Inspired by Gemini CLI's google_web_search tool — enables agents to perform
// grounded Google Searches via the Gemini API and return synthesized answers
// with inline citations and source links.
type Provider struct{}

func (p *Provider) ID() string { return "websearch" }

func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(
		`<circle cx="11" cy="11" r="8"/>` +
			`<path d="m21 21-4.3-4.3"/>` +
			`<path d="M11 8a3 3 0 0 0-3 3"/>`,
	)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "websearch-set",
			Name:        "Web Search",
			Description: "Grounded web search via the Gemini API with real-time results and citations",
			Category:    "Research",
			ToolIDs:     []string{"web_search"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "web_search",
			Name:        "Web Search",
			Description: "Performs a grounded Google Search to find information across the internet. Returns a synthesized answer with citations and source URIs. Best for finding up-to-date documentation, troubleshooting errors, or broad research.",
			Category:    "Research",
			ToolSetID:   "websearch-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

// SettingsSchema declares the per-user settings the web search plugin requires.
// Users configure these via the plugin settings UI.
func (p *Provider) SettingsSchema() []toolregistry.SettingsField {
	return []toolregistry.SettingsField{
		{
			Key:         "api_key",
			Label:       "Gemini API Key",
			Description: "Your Google Gemini API key for grounded web search. Get one at https://aistudio.google.com/apikey",
			Type:        toolregistry.FieldTypePassword,
			Required:    true,
		},
		{
			Key:         "model",
			Label:       "Search Model",
			Description: "The Gemini model used for grounded search queries",
			Type:        toolregistry.FieldTypeSelect,
			Required:    false,
			Default:     "gemini-2.5-flash",
			Options: []string{
				"gemini-2.5-flash",
				"gemini-2.0-flash",
				"gemini-2.0-flash-lite",
				"gemini-1.5-pro",
				"gemini-1.5-flash",
			},
		},
	}
}

func (p *Provider) CreateExecutable(_ context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "web_search":
		// Resolve settings from the per-user plugin settings map.
		var apiKey, model string
		if ps := deps.PluginSettings["websearch"]; ps != nil {
			apiKey = ps["api_key"]
			model = ps["model"]
		}
		return NewWebSearchTool(apiKey, model), nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"web_search"}
}
