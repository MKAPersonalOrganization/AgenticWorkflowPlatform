package toolregistry

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/artifact"
	"context"
	"fmt"
	"sync"
)

// ExecutableToolFactory creates executable tools from tool registry entries and tool sets.
type ExecutableToolFactory struct {
	mu                  sync.RWMutex
	artifactStore       artifact.Service
	toolRegistry        *Registry
	edgequakeBaseURL    string
	openAIAPIKey        string
	modelID             string
	ttsAPIKey           string
	ttsModel            string
	ttsVoice            string
	pluginSettings      map[string]map[string]string // plugin_id → key → value
	allowedKBNamespaces []string                     // workspace IDs the agent may query
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

// SetPluginSettings sets per-user plugin settings resolved from the
// user_plugin_settings table. Outer key is plugin ID, inner key is setting key.
func (f *ExecutableToolFactory) SetPluginSettings(settings map[string]map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pluginSettings = settings
}

// SetAllowedKBNamespaces sets the workspace IDs the agent is permitted to query.
func (f *ExecutableToolFactory) SetAllowedKBNamespaces(namespaces []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowedKBNamespaces = namespaces
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
	deps := &depResolver{factory: f}

	// O(1) lookup via the tool-ID → provider index (covers all built-in tools).
	if provider, ok := LookupProviderForTool(tool.ID); ok {
		executable, err := provider.CreateExecutable(ctx, tool.ID, deps)
		if err != nil {
			return nil, err
		}
		if executable != nil {
			return executable, nil
		}
	}

	// Fallback: linear scan for MCP or dynamically registered tools.
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

// depResolver implements FactoryDeps using lazy, typed getters.
// Dependencies are read from the factory under RLock to ensure thread safety.
type depResolver struct {
	factory *ExecutableToolFactory
}

func (r *depResolver) ArtifactStore() artifact.Service {
	r.factory.mu.RLock()
	defer r.factory.mu.RUnlock()
	return r.factory.artifactStore
}

func (r *depResolver) EdgequakeBaseURL() string {
	r.factory.mu.RLock()
	defer r.factory.mu.RUnlock()
	return r.factory.edgequakeBaseURL
}

func (r *depResolver) OpenAIAPIKey() string {
	r.factory.mu.RLock()
	defer r.factory.mu.RUnlock()
	return r.factory.openAIAPIKey
}

func (r *depResolver) ModelID() string {
	r.factory.mu.RLock()
	defer r.factory.mu.RUnlock()
	return r.factory.modelID
}

func (r *depResolver) TTSConfig() (apiKey, model, voice string) {
	r.factory.mu.RLock()
	defer r.factory.mu.RUnlock()
	apiKey = r.factory.ttsAPIKey
	return apiKey, r.factory.ttsModel, r.factory.ttsVoice
}

func (r *depResolver) PluginSettings(pluginID string) map[string]string {
	r.factory.mu.RLock()
	defer r.factory.mu.RUnlock()
	if r.factory.pluginSettings == nil {
		return nil
	}
	return r.factory.pluginSettings[pluginID]
}

func (r *depResolver) AllowedKBNamespaces() []string {
	r.factory.mu.RLock()
	defer r.factory.mu.RUnlock()
	return r.factory.allowedKBNamespaces
}
