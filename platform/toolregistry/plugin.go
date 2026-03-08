package toolregistry

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/chat/memory"
	"school-with-ai/platform/artifact"

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
type FactoryDeps interface {
	// ArtifactStore returns the artifact storage service.
	ArtifactStore() artifact.Service

	// EdgequakeBaseURL returns the base URL for the Edgequake knowledge base service.
	EdgequakeBaseURL() string

	// OpenAIAPIKey returns the user's configured OpenAI API key.
	OpenAIAPIKey() string

	// ModelID returns the user's configured LLM model ID.
	ModelID() string

	// MemoryService returns a memory service backed by the artifact store.
	// Constructed lazily on first call to avoid allocation when unused.
	MemoryService() memory.Service

	// TTSConfig returns the TTS API key, model, and voice.
	// Falls back to OpenAIAPIKey for the API key if no dedicated TTS key is set.
	TTSConfig() (apiKey, model, voice string)

	// PluginSettings returns per-user settings for the given plugin ID.
	// Returns nil if no settings are configured.
	PluginSettings(pluginID string) map[string]string

	// AllowedKBNamespaces returns workspace IDs the agent may query.
	AllowedKBNamespaces() []string
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
