package toolregistry

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"school-with-ai/foundation/agent"

	"github.com/a-h/templ"
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
	// TTS capability config (resolved per-user from capability settings)
	TTSAPIKey string // API key for the TTS provider (may differ from OpenAIAPIKey)
	TTSModel  string // e.g. "gpt-4o-mini-tts"
	TTSVoice  string // Default voice e.g. "marin", "coral"
}

// BuildIconSVG wraps Lucide-style inner SVG paths in a complete <svg> element.
// The result uses stroke="currentColor" so it inherits the parent's text colour (theme-compatible).
// Width/height are omitted — size the icon via CSS on the container.
func BuildIconSVG(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + inner + `</svg>`
}

// PluginProvider defines the interface that each toolset plugin must implement.
type PluginProvider interface {
	// ID returns the unique identifier for the plugin.
	ID() string

	// Icon returns a complete theme-compatible SVG element representing this plugin.
	// Use [BuildIconSVG] to create one from Lucide inner paths.
	Icon() string

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
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID() < result[j].ID()
	})
	return result
}

// defaultIconSVG is the fallback icon (Lucide "puzzle") for unknown plugins.
var defaultIconSVG = BuildIconSVG(`<path d="M15.39 4.39a1 1 0 0 0 1.68-.474 2.5 2.5 0 1 1 3.014 3.015 1 1 0 0 0-.474 1.68l1.683 1.682a2.414 2.414 0 0 1 0 3.414L19.61 15.39a1 1 0 0 1-1.68-.474 2.5 2.5 0 1 0-3.014 3.015 1 1 0 0 1 .474 1.68l-1.683 1.682a2.414 2.414 0 0 1-3.414 0L8.61 19.61a1 1 0 0 0-1.68.474 2.5 2.5 0 1 1-3.014-3.015 1 1 0 0 0 .474-1.68l-1.683-1.682a2.414 2.414 0 0 1 0-3.414L4.39 8.61a1 1 0 0 1 1.68.474 2.5 2.5 0 1 0 3.014-3.015 1 1 0 0 1-.474-1.68l1.683-1.682a2.414 2.414 0 0 1 3.414 0z"/>`)

// GetPluginIcon returns the full SVG icon for the plugin that owns the given tool set ID.
// Falls back to a puzzle icon if no provider claims the set.
func GetPluginIcon(toolSetID string) string {
	providersMu.RLock()
	defer providersMu.RUnlock()
	for _, p := range providers {
		for _, ts := range p.DefaultSets() {
			if ts.ID == toolSetID {
				return p.Icon()
			}
		}
	}
	return defaultIconSVG
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
