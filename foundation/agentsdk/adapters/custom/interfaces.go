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

// WorkspacePermissionChecker checks workspace access for knowledge-base injection.
// Satisfied by *workspacepermissions.Service.
type WorkspacePermissionChecker interface {
	GetEnabledWorkspaces(ctx context.Context, userID, agentID string) ([]string, error)
}

// ─── Tool Resolution ─────────────────────────────────────────────────────────

// ToolSetFactory creates executable tools from a registered tool set.
// Satisfied by *toolregistry.ExecutableToolFactory.
type ToolSetFactory interface {
	CreateExecutableToolsFromSet(ctx context.Context, toolSetID string) ([]agent.ExecutableTool, error)
	SetAllowedKBNamespaces(namespaces []string)
}

// ToolFactoryConfig carries per-request LLM and TTS configuration for tool creation.
type ToolFactoryConfig struct {
	EdgequakeBaseURL string
	ModelAPIKey      string
	ModelID          string
	TTSAPIKey        string
	TTSModel         string
	TTSVoice         string

	// PluginSettings carries per-user, per-plugin settings resolved from
	// the user_plugin_settings table. Outer key = plugin ID, inner key =
	// setting key, value = setting value.
	PluginSettings map[string]map[string]string

	// AllowedKBNamespaces is the set of workspace IDs the agent may query.
	AllowedKBNamespaces []string
}

// NewToolSetFactoryFunc builds a ToolSetFactory with per-request configuration.
// Injected by the composition root so the adapter never imports service packages.
type NewToolSetFactoryFunc func(cfg ToolFactoryConfig) ToolSetFactory

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

// SkillPromptInjector provides per-thread skill instructions for system prompt injection.
// Satisfied by *skills.SkillActivationTracker.
type SkillPromptInjector interface {
	// BuildPromptAddition returns activated skill instructions for the given thread,
	// formatted for direct concatenation with the system prompt.
	// Returns empty string if no skills are activated.
	BuildPromptAddition(threadID string) string
}
