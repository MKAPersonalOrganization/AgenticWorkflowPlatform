package toolregistry

import (
	"context"
	"fmt"
	"school-with-ai/foundation/agent"
	"school-with-ai/services/artifact"
	"school-with-ai/services/memory"
	"sync"
)

// ExecutableToolFactory creates executable tools from tool registry entries and tool sets.
type ExecutableToolFactory struct {
	mu               sync.RWMutex
	artifactStore    artifact.Service
	toolRegistry     *Registry
	edgequakeBaseURL string
	openAIAPIKey     string
	modelID          string
	ttsAPIKey        string
	ttsModel         string
	ttsVoice         string
}

// NewExecutableToolFactory creates a new tool factory.
func NewExecutableToolFactory(artifactStore artifact.Service) *ExecutableToolFactory {
	return &ExecutableToolFactory{
		artifactStore: artifactStore,
	}
}

// SetEdgequakeConfig sets the edgequake base URL and LLM config for knowledge base tools.
// The openAIAPIKey and modelID are used by the KB sub-agent to synthesize answers.
func (f *ExecutableToolFactory) SetEdgequakeConfig(edgequakeBaseURL, openAIAPIKey, modelID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.edgequakeBaseURL = edgequakeBaseURL
	f.openAIAPIKey = openAIAPIKey
	f.modelID = modelID
}

// SetTTSConfig sets the TTS model, voice, and API key resolved from user's capability config.
func (f *ExecutableToolFactory) SetTTSConfig(apiKey, model, voice string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ttsAPIKey = apiKey
	f.ttsModel = model
	f.ttsVoice = voice
}

// SetToolRegistry sets the tool registry for the factory (used for tool set expansion).
func (f *ExecutableToolFactory) SetToolRegistry(registry *Registry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.toolRegistry = registry
}

// CreateExecutableToolsFromSet creates all executable tools from a tool set.
func (f *ExecutableToolFactory) CreateExecutableToolsFromSet(ctx context.Context, toolSetID string) ([]agent.ExecutableTool, error) {
	f.mu.RLock()
	registry := f.toolRegistry
	f.mu.RUnlock()

	if registry == nil {
		return nil, fmt.Errorf("tool registry not set")
	}

	// Get tools in the set
	tools, err := registry.GetToolsInSet(ctx, toolSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools in set %s: %w", toolSetID, err)
	}

	var executableTools []agent.ExecutableTool
	for _, tool := range tools {
		executableTool, err := f.CreateExecutableTool(ctx, &tool)
		if err != nil {
			// Skip tools that can't be created rather than failing the entire set
			continue
		}
		executableTools = append(executableTools, executableTool)
	}

	return executableTools, nil
}

// CreateExecutableTool creates an executable tool from a tool registry entry.
func (f *ExecutableToolFactory) CreateExecutableTool(ctx context.Context, tool *Tool) (agent.ExecutableTool, error) {
	f.mu.RLock()
	ttsAPIKey := f.ttsAPIKey
	if ttsAPIKey == "" {
		ttsAPIKey = f.openAIAPIKey // fallback to LLM key for backward compat
	}
	deps := FactoryDeps{
		ArtifactStore:    f.artifactStore,
		EdgequakeBaseURL: f.edgequakeBaseURL,
		OpenAIAPIKey:     f.openAIAPIKey,
		ModelID:          f.modelID,
		MemoryService:    memory.NewArtifactMemoryService(f.artifactStore),
		TTSAPIKey:        ttsAPIKey,
		TTSModel:         f.ttsModel,
		TTSVoice:         f.ttsVoice,
	}
	f.mu.RUnlock()

	for _, provider := range GetAllPluginProviders() {
		executable, err := provider.CreateExecutable(ctx, tool.ID, deps)
		if err != nil {
			return nil, err
		}
		if executable != nil {
			return executable, nil
		}
	}

	return nil, fmt.Errorf("unknown tool ID (no plugin handled it): %s", tool.ID)
}
