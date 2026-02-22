package toolregistry

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/a-h/templ"

	"school-with-ai/foundation/agent"
)

// ToolUIContext provides the data required for a plugin to render its UI.
type ToolUIContext struct {
	MessageID    string
	MessageName  string
	ParsedArgs   any
	ParsedResult map[string]any
	RawResult    string
	Error        bool
}

// FactoryDeps holds the dependencies required to instantiate certain tools.
type FactoryDeps struct {
	ArtifactStore    any // To be type-asserted by the specific plugin if needed
	EdgequakeBaseURL string
	OpenAIAPIKey     string
	ModelID          string
	MemoryService    any
}

// PluginProvider defines the interface that each toolset plugin must implement.
type PluginProvider interface {
	// ID returns the unique identifier for the plugin.
	ID() string

	// DefaultSets returns the default tool sets provided by this plugin.
	DefaultSets() []ToolSet

	// DefaultTools returns the default tools provided by this plugin.
	DefaultTools() []Tool

	// CreateExecutable creates an executable tool instance given a tool ID.
	CreateExecutable(ctx context.Context, toolID string, deps FactoryDeps) (agent.ExecutableTool, error)

	// GetUITemplate returns the Templ component containing the Alpine <template x-if="..."> blocks.
	// If the plugin does not provide UI components, it should return nil.
	GetUITemplate() templ.Component

	// UIHandledTools returns the list of tool message names this plugin renders custom UI for.
	UIHandledTools() []string

	// UIHandledElicitations returns the list of elicitation schema types this plugin provides UI for.
	UIHandledElicitations() []string
}

var (
	providersMu sync.RWMutex
	providers   = make(map[string]PluginProvider)
)

// RegisterPlugin registers a new plugin provider.
func RegisterPlugin(p PluginProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.ID()] = p
}

// GetPluginProvider returns a plugin provider by ID.
func GetPluginProvider(id string) (PluginProvider, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[id]
	return p, ok
}

// GetAllPluginProviders returns all registered plugin providers.
func GetAllPluginProviders() []PluginProvider {
	providersMu.RLock()
	defer providersMu.RUnlock()
	result := make([]PluginProvider, 0, len(providers))
	for _, p := range providers {
		result = append(result, p)
	}
	return result
}

// GetUIHandledToolsJSON returns a JSON-encoded list of all tool names that possess a custom UI template.
func GetUIHandledToolsJSON() string {
	var handled []string
	for _, p := range GetAllPluginProviders() {
		handled = append(handled, p.UIHandledTools()...)
	}
	// Inject any hardcoded stragglers that need special state routing but lack a literal visual component
	handled = append(handled, "transfer_to_agent")

	bytes, _ := json.Marshal(handled)
	return string(bytes)
}

// GetUIHandledElicitationsJSON returns a JSON-encoded list of all elicitation schema types that possess a custom UI template.
func GetUIHandledElicitationsJSON() string {
	var handled []string
	for _, p := range GetAllPluginProviders() {
		handled = append(handled, p.UIHandledElicitations()...)
	}

	bytes, _ := json.Marshal(handled)
	return string(bytes)
}
