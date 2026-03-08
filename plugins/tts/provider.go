package tts

import (
	"context"
	"time"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/artifact"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
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

// SettingsSchema declares the per-user settings the TTS plugin supports.
// Users can optionally override the API key, model, and voice here instead of
// relying on the AI-provider capability system.
func (p *Provider) SettingsSchema() []toolregistry.SettingsField {
	return []toolregistry.SettingsField{
		{
			Key:         "api_key",
			Label:       "OpenAI API Key",
			Description: "Your OpenAI API key for text-to-speech. Leave empty to use the key from AI Settings.",
			Type:        toolregistry.FieldTypePassword,
			Required:    false,
		},
		{
			Key:         "model",
			Label:       "TTS Model",
			Description: "The OpenAI model used for speech generation",
			Type:        toolregistry.FieldTypeSelect,
			Required:    false,
			Default:     "gpt-4o-mini-tts",
			Options:     []string{"gpt-4o-mini-tts", "tts-1", "tts-1-hd"},
		},
		{
			Key:         "voice",
			Label:       "Default Voice",
			Description: "The default voice for speech generation. Can be overridden per request.",
			Type:        toolregistry.FieldTypeSelect,
			Required:    false,
			Default:     "coral",
			Options:     []string{"alloy", "ash", "ballad", "coral", "echo", "fable", "nova", "onyx", "sage", "shimmer", "verse", "marin", "cedar"},
		},
	}
}

func (p *Provider) CreateExecutable(ctx context.Context, toolID string, deps toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	switch toolID {
	case "tts-speak":
		artifactStore, _ := deps.ArtifactStore.(artifact.Service)

		// Resolve settings: plugin settings override → TTS capability config → OpenAI key fallback
		apiKey := deps.TTSAPIKey
		model := deps.TTSModel
		voice := deps.TTSVoice

		if ps := deps.PluginSettings["tts"]; ps != nil {
			if v := ps["api_key"]; v != "" {
				apiKey = v
			}
			if v := ps["model"]; v != "" {
				model = v
			}
			if v := ps["voice"]; v != "" {
				voice = v
			}
		}

		if apiKey == "" {
			apiKey = deps.OpenAIAPIKey // backward compat
		}

		return NewSpeakTool(artifactStore, apiKey, model, voice), nil
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
