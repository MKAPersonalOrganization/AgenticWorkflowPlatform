// Package adkgo provides the adapter layer that bridges Google's Agent
// Development Kit (ADK-Go) with the application's chat module.
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

// AgentProvider retrieves agent definitions for a user.
type AgentProvider interface {
	GetForUser(ctx context.Context, name, userID string) (agent.Agent, error)
}

// AgentLister is an optional extension of AgentProvider that supports
// listing all available agents. Providers that implement this interface
// enable the ListAgents functionality in the RunnerAdapter.
type AgentLister interface {
	List() []agent.Agent
}

// ToolSetFactory creates executable tools from a registered tool set.
type ToolSetFactory interface {
	CreateExecutableToolsFromSet(ctx context.Context, toolSetID string) ([]agent.ExecutableTool, error)
}

// ToolFactoryConfig carries per-request configuration for tool creation.
// Tool-specific settings (API keys, service URLs) are passed via the Extra map
// so the adapter layer remains tool-agnostic.
type ToolFactoryConfig struct {
	ModelAPIKey    string
	ModelID        string
	PluginSettings map[string]map[string]string
	Extra          map[string]string // Additional per-request config (e.g., service URLs, API keys)
}

// NewToolSetFactoryFunc creates a ToolSetFactory scoped to a single request.
type NewToolSetFactoryFunc func(cfg ToolFactoryConfig) ToolSetFactory

// ExtraConfigFunc resolves per-user, per-agent, per-request tool configuration.
// The returned map is passed as ToolFactoryConfig.Extra so the adapter
// layer remains tool-agnostic while the composition root can provide
// tool-specific settings (API keys, service URLs, etc.).
type ExtraConfigFunc func(ctx context.Context, userID, agentName string) map[string]string

// ToolRequirementsService is the exclusive authority for all tool access
// decisions. It unifies permission checks, user confirmation, and any future
// requirement types into a single evaluation pipeline.
//
// The adapter uses this for:
//   - Tool resolution: GetEnabledToolSets, HasToolSetPermission, HasPermission
//   - Confirmation: ConfirmationMiddleware (injected into the tool execution chain)
type ToolRequirementsService interface {
	GetEnabledToolSets(ctx context.Context, userID, agentName string) ([]string, error)
	HasToolSetPermission(ctx context.Context, userID, agentName, toolSetID string) bool
	HasPermission(ctx context.Context, userID, agentName, toolSetID, toolID string) bool

	// ConfirmationMiddleware returns middleware enforcing user confirmation
	// for the given agent.
	ConfirmationMiddleware(agentID string) agent.Middleware
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

// A2AToolResolver resolves A2A-based tools for a user.
type A2AToolResolver interface {
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

// PromptInjector injects additional prompt content at system-prompt level.
type PromptInjector interface {
	BuildPromptAddition(ctx context.Context, agentName, threadID string) string
}

// UserContextProvider provides user context information for prompt injection.
type UserContextProvider interface {
	GetProfile(ctx context.Context, userID string) (map[string]any, error)
}
