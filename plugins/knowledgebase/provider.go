package knowledgebase

import (
	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"
	"context"
	"time"

	"github.com/a-h/templ"
)

// Provider implements the toolregistry.PluginProvider interface for the Knowledge Base toolset.
type Provider struct{}

func (p *Provider) ID() string { return "knowledgebase" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<path d="M12 7v14"/><path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z"/>`)
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "knowledge-base-set",
			Name:        "Knowledge Base",
			Description: "Tools for querying attached knowledge base workspaces",
			Category:    "Knowledge",
			ToolIDs:     []string{"query_knowledge_base"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{ID: "query_knowledge_base", Name: "Query Knowledge Base", Description: "Search and retrieve information from attached knowledge base workspaces", Category: "Knowledge", ToolSetID: "knowledge-base-set", CreatedAt: now, UpdatedAt: now},
	}
}

// SettingsSchema declares the per-user settings for the knowledge base plugin.
// Users can optionally provide their own OpenAI key and model for the KB sub-agent.
func (p *Provider) SettingsSchema() []toolregistry.SettingsField {
	return []toolregistry.SettingsField{
		{
			Key:         "api_key",
			Label:       "OpenAI API Key",
			Description: "Your OpenAI API key for the knowledge-base sub-agent. Leave empty to use the key from AI Settings.",
			Type:        toolregistry.FieldTypePassword,
			Required:    false,
		},
		{
			Key:         "model",
			Label:       "Synthesis Model",
			Description: "The OpenAI model used to synthesize answers from retrieved knowledge",
			Type:        toolregistry.FieldTypeSelect,
			Required:    false,
			Default:     "gpt-4o",
			Options:     []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "o4-mini"},
		},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "query_knowledge_base":
		// Resolve settings: plugin settings override → deps fallback
		apiKey := deps.OpenAIAPIKey()
		modelID := deps.ModelID()

		if ps := deps.PluginSettings("knowledgebase"); ps != nil {
			if v := ps["api_key"]; v != "" {
				apiKey = v
			}
			if v := ps["model"]; v != "" {
				modelID = v
			}
		}

		return NewQueryKnowledgeBaseTool(deps.EdgequakeBaseURL(), apiKey, modelID, deps.AllowedKBNamespaces()), nil
	default:
		return nil, nil // Not handled by this plugin
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"query_knowledge_base"}
}
