package custom

import (
	"context"
	"iter"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/chat/thread"
)

// ─── Thread Management ──────────────────────────────────────────────────────

// ThreadManager abstracts thread lifecycle and message persistence.
// Satisfied by *threadmgmt.Manager — the custom package never imports the
// concrete type, keeping the dependency arrow from module → foundation.
type ThreadManager interface {
	AddUserMessage(ctx context.Context, threadID, messageID, content, userID string, attachments ...thread.Attachment) (*thread.ThreadAggregate, *thread.UserMessageAdded, []thread.Event, error)
	Load(ctx context.Context, threadID string) (*thread.ThreadAggregate, error)
}

// EventProcessor processes and broadcasts domain events.
type EventProcessor interface {
	ProcessEvents(ctx context.Context, events iter.Seq2[thread.Event, error]) iter.Seq2[thread.Event, error]
}

// ─── Permissions ─────────────────────────────────────────────────────────────

// ToolPermissionChecker checks agent-level tool and tool-set permissions.
// Satisfied by *permissions.Service.
type ToolPermissionChecker interface {
	GetEnabledToolSets(ctx context.Context, userID, agentID string) ([]string, error)
	HasToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) bool
	HasPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) bool
}

// ─── Tool Resolution ─────────────────────────────────────────────────────────

// ToolSetFactory creates executable tools from a registered tool set.
// Satisfied by *toolregistry.ExecutableToolFactory.
type ToolSetFactory interface {
	CreateExecutableToolsFromSet(ctx context.Context, toolSetID string) ([]agent.ExecutableTool, error)
}

// ToolFactoryConfig carries per-request configuration for tool creation.
// Tool-specific settings (API keys, service URLs) are passed via the Extra map
// so the adapter layer remains tool-agnostic.
type ToolFactoryConfig struct {
	ModelAPIKey string
	ModelID     string

	// PluginSettings carries per-user, per-plugin settings resolved from
	// the user_plugin_settings table. Outer key = plugin ID, inner key =
	// setting key, value = setting value.
	PluginSettings map[string]map[string]string

	// Extra carries additional per-request config (e.g., service URLs, API keys)
	// so the adapter layer does not need tool-specific fields.
	Extra map[string]string
}

// NewToolSetFactoryFunc builds a ToolSetFactory with per-request configuration.
// Injected by the composition root so the adapter never imports service packages.
type NewToolSetFactoryFunc func(cfg ToolFactoryConfig) ToolSetFactory

// ExtraConfigFunc resolves per-user, per-agent, per-request tool configuration.
// The returned map is passed as ToolFactoryConfig.Extra so the adapter
// layer remains tool-agnostic while the composition root can provide
// tool-specific settings (API keys, service URLs, etc.).
type ExtraConfigFunc func(ctx context.Context, userID, agentName string) map[string]string

// MCPToolResolver resolves MCP-connected tools for a user.
// Satisfied by *mcp.Service.
type MCPToolResolver interface {
	ResolveEnabledTools(ctx context.Context, userID string,
		isToolSetEnabled func(toolSetID string) bool,
		isToolEnabled func(toolSetID, toolID string) bool,
	) []agent.ExecutableTool
}

// PluginSettingsResolver resolves per-user plugin settings at request time.
// Satisfied by *pluginsettings.Module.
type PluginSettingsResolver interface {
	// GetAllPluginSettingsForUser returns all plugin settings as a map for a user.
	// Outer key = plugin ID, inner key = setting key, value = setting value.
	GetAllPluginSettingsForUser(ctx context.Context, userID string) (map[string]map[string]string, error)
}

// PromptInjector provides per-request prompt additions for system prompt injection.
type PromptInjector interface {
	// BuildPromptAddition returns additional instructions for the given agent and thread,
	// formatted for direct concatenation with the system prompt.
	// Returns empty string if no additions are needed.
	BuildPromptAddition(ctx context.Context, agentName, threadID string) string
}
