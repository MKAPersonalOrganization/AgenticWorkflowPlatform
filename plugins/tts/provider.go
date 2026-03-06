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

func (p *Provider) ID() string { return "tts" }
func (p *Provider) Icon() string {
	return toolregistry.BuildIconSVG(`<path d="M11 4.702a.705.705 0 0 0-1.203-.498L6.413 7.587A1.4 1.4 0 0 1 5.416 8H3a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2.416a1.4 1.4 0 0 1 .997.413l3.383 3.384A.705.705 0 0 0 11 19.298z"/><path d="M16 9a5 5 0 0 1 0 6"/><path d="M19.364 18.364a9 9 0 0 0 0-12.728"/>`)
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
		// Use TTS-specific API key/model/voice from user's capability config
		apiKey := deps.TTSAPIKey
		if apiKey == "" {
			apiKey = deps.OpenAIAPIKey // backward compat
		}
		return NewSpeakTool(artifactStore, apiKey, deps.TTSModel, deps.TTSVoice), nil
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
