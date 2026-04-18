package toolregistry

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/artifact"
	"context"
	"encoding/json"
	"sort"
	"sync"

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

// SettingsFieldType enumerates allowed plugin-setting field types.
type SettingsFieldType string

const (
	FieldTypeString   SettingsFieldType = "string"   // Plain text
	FieldTypePassword SettingsFieldType = "password" // Masked / sensitive
	FieldTypeSelect   SettingsFieldType = "select"   // Dropdown with Options
)

// SettingsField describes one user-configurable setting that a plugin exposes.
// Plugins declare their schema via [PluginProvider.SettingsSchema]; the platform
// validates and persists the values per-user-per-plugin.
type SettingsField struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Description string            `json:"description,omitempty"`
	Type        SettingsFieldType `json:"type"`
	Required    bool              `json:"required"`
	Default     string            `json:"default,omitempty"`
	Options     []string          `json:"options,omitempty"` // Used when Type == FieldTypeSelect
}

// FactoryDeps provides lazy, typed access to tool creation dependencies.
// Plugins call only the getters they need, making dependencies explicit and
// avoiding unnecessary allocations for services the requesting plugin doesn't use.
//
// # Plugin usage matrix (14 plugins, 2025-07)
//
// Only 7 plugins actively consume FactoryDeps; the other 8 ignore it entirely.
//
//	knowledgebase → EdgequakeBaseURL, OpenAIAPIKey, ModelID, PluginSettings, AllowedKBNamespaces
//	tts           → ArtifactStore, OpenAIAPIKey, TTSConfig, PluginSettings
//	document      → ArtifactStore
//	websearch     → PluginSettings, GroundedSearchClientFactory
//	fsplugin      → AgentFilesystem
//	artifacts, calculator, codebase, cognitive, demoinput, library, planmode, todos, tracker → (none)
//
// The interface is intentionally kept as a single composite rather than split
// into narrow per-concern interfaces. With only 10 methods and 14 plugins, the
// ISP trade-off favours simplicity. If many more plugins are added, consider
// splitting into ArtifactProvider, KBConfigProvider, etc.
type FactoryDeps interface {
	// ArtifactStore returns the artifact storage service.
	// Used by: document, tts.
	ArtifactStore() artifact.Service

	// EdgequakeBaseURL returns the base URL for the Edgequake knowledge base service.
	// Used by: knowledgebase.
	EdgequakeBaseURL() string

	// OpenAIAPIKey returns the user's configured OpenAI API key.
	// Used by: knowledgebase, tts (as fallback for TTS API key).
	OpenAIAPIKey() string

	// ModelID returns the user's configured LLM model ID.
	// Used by: knowledgebase.
	ModelID() string

	// TTSConfig returns the TTS API key, model, and voice.
	// Falls back to OpenAIAPIKey for the API key if no dedicated TTS key is set.
	// Used by: tts.
	TTSConfig() (apiKey, model, voice string)

	// PluginSettings returns per-user settings for the given plugin ID.
	// Returns nil if no settings are configured.
	// Used by: knowledgebase, tts, websearch.
	PluginSettings(pluginID string) map[string]string

	// AllowedKBNamespaces returns workspace IDs the agent may query.
	// Used by: knowledgebase.
	AllowedKBNamespaces() []string

	// GroundedSearchClientFactory returns a lazy factory for web-search clients.
	//
	// The return type is intentionally `any` to avoid an import cycle between
	// foundation/toolregistry and plugins/websearch. The websearch plugin is
	// responsible for asserting the concrete client type it expects.
	GroundedSearchClientFactory() func(apiKey, model string) any

	// AgentFilesystem returns the filesystem dependency for fsplugin tools.
	//
	// The return type is intentionally `any` to avoid an import cycle between
	// foundation/toolregistry and plugins/fsplugin. The fsplugin package is
	// responsible for asserting the concrete interface it expects.
	AgentFilesystem() any
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

	// SettingsSchema returns the list of user-configurable settings this plugin
	// requires. Return nil or an empty slice if the plugin has no settings.
	// The platform renders a settings form from this schema and stores the
	// values per-user, per-plugin in the user_plugin_settings table.
	SettingsSchema() []SettingsField

	// CreateExecutable creates an executable tool instance given a tool ID.
	CreateExecutable(ctx context.Context, toolID string, deps FactoryDeps) (agent.ExecutableTool, error)

	// GetUITemplate returns the Templ component containing the Alpine <template x-if="..."> blocks.
	// If the plugin does not provide UI components, it should return nil.
	GetUITemplate() templ.Component

	// UIHandledTools returns the list of tool message names this plugin renders custom UI for.
	UIHandledTools() []string
}

var (
	providersMu    sync.RWMutex
	providers      = make(map[string]PluginProvider)
	toolToProvider = make(map[string]string) // toolID → providerID index
)

// RegisterPlugin registers a new plugin provider and indexes its tool IDs.
func RegisterPlugin(p PluginProvider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers[p.ID()] = p
	for _, tool := range p.DefaultTools() {
		toolToProvider[tool.ID] = p.ID()
	}
}

// LookupProviderForTool returns the provider that owns the given tool ID.
// Returns nil, false for MCP or dynamically registered tools.
func LookupProviderForTool(toolID string) (PluginProvider, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	pid, ok := toolToProvider[toolID]
	if !ok {
		return nil, false
	}
	p, ok := providers[pid]
	return p, ok
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

// GetPluginIDForSet returns the ID of the plugin that owns the given tool set ID.
// Returns an empty string if no provider claims the set.
func GetPluginIDForSet(toolSetID string) string {
	providersMu.RLock()
	defer providersMu.RUnlock()
	for _, p := range providers {
		for _, ts := range p.DefaultSets() {
			if ts.ID == toolSetID {
				return p.ID()
			}
		}
	}
	return ""
}

// GetUIHandledToolsJSON returns a JSON-encoded list of all tool names that possess a custom UI template.
func GetUIHandledToolsJSON() string {
	var handled []string
	for _, p := range GetAllPluginProviders() {
		handled = append(handled, p.UIHandledTools()...)
	}
	bytes, _ := json.Marshal(handled)
	return string(bytes)
}

// PluginSettingsMeta is a lightweight summary of a plugin's configurable settings.
type PluginSettingsMeta struct {
	PluginID string          `json:"plugin_id"`
	Icon     string          `json:"icon"`
	Schema   []SettingsField `json:"schema"`
}

// GetAllPluginSettingsSchemas returns schema metadata for every registered
// plugin that declares at least one SettingsField.
func GetAllPluginSettingsSchemas() []PluginSettingsMeta {
	var out []PluginSettingsMeta
	for _, p := range GetAllPluginProviders() {
		schema := p.SettingsSchema()
		if len(schema) == 0 {
			continue
		}
		out = append(out, PluginSettingsMeta{
			PluginID: p.ID(),
			Icon:     p.Icon(),
			Schema:   schema,
		})
	}
	return out
}
