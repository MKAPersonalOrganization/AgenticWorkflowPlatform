// Package adkgo provides the adapter layer that bridges Google's Agent
// Development Kit (ADK-Go) with the application's chat module.
package adkgo

import (
	"aicompass/foundation/agent"
	"aicompass/modules/chat/thread"
	"context"
	"iter"
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

// APIKeyResolver resolves the API key for a specific provider for a user.
// Used when a per-agent model override selects a provider different from
// the user's global default.
type APIKeyResolver interface {
	GetAPIKeyForProvider(ctx context.Context, userID, provider string) (string, error)
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

// ToolRequirementsService is the authority for tool access decisions
// during tool resolution. It determines which tools are visible to the LLM.
//
// The adapter uses this for:
//   - Tool resolution: GetEnabledToolSets, HasToolSetPermission, HasPermission
type ToolRequirementsService interface {
	GetEnabledToolSets(ctx context.Context, userID, agentName string) ([]string, error)
	HasToolSetPermission(ctx context.Context, userID, agentName, toolSetID string) bool
	HasPermission(ctx context.Context, userID, agentName, toolSetID, toolID string) bool
}

// BlockEnforcer provides runtime enforcement for blocked tools.
// This is a defense-in-depth safety net — tools should already be filtered at
// resolution time, but this catches race conditions (config changed mid-conversation).
type BlockEnforcer interface {
	// BlockMiddleware returns middleware that prevents execution of tools
	// the user has blocked.
	BlockMiddleware(agentID string) agent.Middleware
}

// ConfirmationChecker determines whether a tool call needs user confirmation.
// It checks per-user DB config and admin-level global policies.
// Implemented by toolreqs.Service.
type ConfirmationChecker interface {
	CheckConfirmation(ctx context.Context, userID, agentID, toolSetID, toolName string, args map[string]any) (bool, string)
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

// A2AConnectorInfo carries the metadata needed to describe a remote A2A agent
// as a workflow sub-agent.
type A2AConnectorInfo struct {
	ID          string
	Name        string
	Description string
	AgentURL    string
}

// A2AStreamChunk is a single unit yielded by A2ASubAgentProvider.SendStreamingMessage.
//
// Exactly one kind is set per chunk:
//   - Delta != "":  incremental text streaming delta
//   - IsArtifact:   complete text artifact (rendered as a document card)
//   - IsFile:       file attachment (download card; URL may be remote or data-URI)
type A2AStreamChunk struct {
	// Text delta (working status-update streaming)
	Delta string

	// Text artifact (artifact-update with text content)
	IsArtifact bool
	ArtifactID string
	Title      string
	Content    string

	// File attachment (artifact-update or message with file/URL parts)
	IsFile   bool
	FileURL  string // remote URI or data:[mimeType];base64,... for inline bytes
	FileName string
	MimeType string
}

// A2ASubAgentProvider resolves A2A connectors for use as workflow sub-agents.
// This is distinct from A2AToolResolver (which exposes connectors as tools)
// because sub-agents delegate an entire turn rather than a single tool call.
type A2ASubAgentProvider interface {
	// GetConnectorInfo retrieves the metadata for a connector, scoped to user.
	GetConnectorInfo(ctx context.Context, connectorID, userID string) (A2AConnectorInfo, error)
	// SendMessage sends a message to the remote agent and returns its complete response.
	SendMessage(ctx context.Context, connectorID, userID, message string) (string, error)
	// SendStreamingMessage streams chunks from the remote agent as they arrive.
	// Text delta chunks (Delta != "") carry incremental LLM output; artifact chunks
	// (IsArtifact=true) carry complete remote artifacts for separate display.
	SendStreamingMessage(ctx context.Context, connectorID, userID, message string) iter.Seq2[A2AStreamChunk, error]
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
