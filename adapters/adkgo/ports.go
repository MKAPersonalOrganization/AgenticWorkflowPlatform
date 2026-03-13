// Package adkgo provides an adapter layer that bridges Google's Agent Development
// Kit (ADK-Go) with SchoolWithAI's chat module. It replaces the former custom
// engine adapter (adapters/custom) and the event-sourcing infrastructure with a
// simpler, ADK-Go–native execution model.
package adkgo

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/chat/thread"
)

// ──────────────────────────────────────────────────────────────────────────────
// Narrow interfaces – only the capabilities the adapter actually needs.
// ──────────────────────────────────────────────────────────────────────────────

// ThreadManager manages the mutable side of conversation threads.
type ThreadManager interface {
	// AddUserMessage persists a user message and returns the updated aggregate
	// plus the emitted events.
	AddUserMessage(
		ctx context.Context,
		threadID, messageID, content, userID string,
		attachments ...thread.Attachment,
	) (*thread.ThreadAggregate, *thread.UserMessageAdded, []thread.Event, error)

	// Load reconstructs a thread aggregate from the event store.
	Load(ctx context.Context, threadID string) (*thread.ThreadAggregate, error)
}

// ModelConfigResolver resolves per-user LLM configuration.
type ModelConfigResolver interface {
	GetModelConfigForUser(ctx context.Context, userID string) (*agent.ModelConfig, error)
}

// TTSConfigResolver resolves per-user text-to-speech configuration.
type TTSConfigResolver interface {
	GetTTSConfigForUserFlat(ctx context.Context, userID string) (apiKey, model, voice string, err error)
}

// AgentProvider retrieves agent definitions for a user.
type AgentProvider interface {
	GetForUser(ctx context.Context, name, userID string) (agent.Agent, error)
}

// ToolSetFactory creates executable tools from a registered tool set.
type ToolSetFactory interface {
	CreateExecutableToolsFromSet(ctx context.Context, toolSetID string) ([]agent.ExecutableTool, error)
	SetAllowedKBNamespaces(namespaces []string)
}

// ToolFactoryConfig carries per-request configuration for tool creation.
type ToolFactoryConfig struct {
	EdgequakeBaseURL    string
	ModelAPIKey         string
	ModelID             string
	TTSAPIKey           string
	TTSModel            string
	TTSVoice            string
	PluginSettings      map[string]map[string]string
	AllowedKBNamespaces []string
}

// NewToolSetFactoryFunc creates a ToolSetFactory scoped to a single request.
type NewToolSetFactoryFunc func(cfg ToolFactoryConfig) ToolSetFactory

// ToolPermissionChecker provides per-user, per-agent tool access control.
type ToolPermissionChecker interface {
	GetEnabledToolSets(ctx context.Context, userID, agentName string) ([]string, error)
	HasToolSetPermission(ctx context.Context, userID, agentName, toolSetID string) bool
	HasPermission(ctx context.Context, userID, agentName, toolSetID, toolID string) bool
}

// WorkspacePermissionChecker provides per-user, per-agent workspace access control.
type WorkspacePermissionChecker interface {
	GetEnabledWorkspaces(ctx context.Context, userID, agentName string) ([]string, error)
}

// MCPToolResolver resolves MCP-based tools for a user.
type MCPToolResolver interface {
	ResolveEnabledTools(
		ctx context.Context,
		userID string,
		hasToolSetPerm func(toolSetID string) bool,
		hasToolPerm func(toolSetID, toolID string) bool,
	) []agent.ExecutableTool
}

// PluginSettingsResolver provides per-user plugin settings.
type PluginSettingsResolver interface {
	GetAllPluginSettingsForUser(ctx context.Context, userID string) (map[string]map[string]string, error)
}

// SkillPromptInjector injects skill-specific prompt additions.
type SkillPromptInjector interface {
	BuildPromptAddition(threadID string) string
}

// StudentProfileProvider provides student profile information for prompt injection.
type StudentProfileProvider interface {
	GetProfile(ctx context.Context, userID string) (map[string]any, error)
}
