package tts

import (
	"context"
	"time"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/artifact"
	"school-with-ai/services/toolregistry"
)

// Provider implements the toolregistry.PluginProvider interface for the TTS toolset.
type Provider struct{}

func (p *Provider) ID() string {
	return "tts"
}

func (p *Provider) DefaultSets() []toolregistry.ToolSet {
	now := time.Now()
	return []toolregistry.ToolSet{
		{
			ID:          "tts-set",
			Name:        "Text to Speech",
			Description: "Generate spoken audio from text using AI voices",
			Category:    "Audio",
			ToolIDs:     []string{"tts-speak"},
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) DefaultTools() []toolregistry.Tool {
	now := time.Now()
	return []toolregistry.Tool{
		{
			ID:          "tts-speak",
			Name:        "Speak",
			Description: "Convert text to spoken audio using AI voices",
			Category:    "Audio",
			ToolSetID:   "tts-set",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "tts-speak":
		artifactStore, _ := deps.ArtifactStore.(artifact.Service)
		return NewSpeakTool(artifactStore, deps.OpenAIAPIKey), nil
	default:
		return nil, nil
	}
}

func (p *Provider) GetUITemplate() templ.Component {
	return PluginTemplate()
}

func (p *Provider) UIHandledTools() []string {
	return []string{"speak", "tts-speak"}
}

func (p *Provider) UIHandledElicitations() []string {
	return nil
}
