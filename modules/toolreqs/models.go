package toolreqs

import (
	"encoding/json"
	"time"
)

// =============================================================================
// Requirement configuration — per-user, per-agent, per-tool requirements
// =============================================================================

// RequirementAction controls whether a tool is allowed, blocked, or requires
// user confirmation before execution. Stored per-user, per-agent,
// per-tool (or per-tool-set) and consulted at runtime by [Service].
type RequirementAction string

const (
	// RequirementActionAllow means the tool is permitted (default).
	RequirementActionAllow RequirementAction = "allow"

	// RequirementActionBlock means the tool is hard-blocked and will not be
	// visible to the LLM.
	RequirementActionBlock RequirementAction = "block"

	// RequirementActionRequireConfirmation means the tool requires explicit
	// user approval before every invocation (HITL / human-in-the-loop).
	RequirementActionRequireConfirmation RequirementAction = "require_confirmation"
)

// ToolRequirementConfig holds per-user, per-agent requirement settings for a
// single tool or tool set. It is serialised as JSON into the `config` column
// of the permission tables.
//
// The zero value is a valid "no special requirements" config: action defaults
// to [RequirementActionAllow], no confirmation hint.
type ToolRequirementConfig struct {
	// Action governs the top-level requirement: allow / block / require_confirmation.
	Action RequirementAction `json:"action,omitempty"`

	// RequireConfirmation is an explicit flag equivalent to
	// Action == RequirementActionRequireConfirmation. When true, every
	// invocation pauses for user approval. Maps to ADK-Go's
	// FunctionTool.Config.RequireConfirmation.
	RequireConfirmation bool `json:"require_confirmation,omitempty"`

	// ConfirmationHint is the human-readable prompt shown to the user when
	// confirmation is required. Maps to ADK-Go ToolConfirmation.Hint.
	ConfirmationHint string `json:"confirmation_hint,omitempty"`
}

// ParseToolRequirementConfig deserialises a JSON config string into a
// [ToolRequirementConfig]. Returns a zero-value config on empty/invalid input.
func ParseToolRequirementConfig(raw string) ToolRequirementConfig {
	if raw == "" || raw == "{}" {
		return ToolRequirementConfig{}
	}
	var cfg ToolRequirementConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return ToolRequirementConfig{}
	}
	return cfg
}

// MarshalConfig serialises the config to a JSON string suitable for storage.
func (c ToolRequirementConfig) MarshalConfig() string {
	b, err := json.Marshal(c)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// IsEmpty returns true when the config carries no meaningful requirements.
func (c ToolRequirementConfig) IsEmpty() bool {
	return c.Action == "" && !c.RequireConfirmation && c.ConfirmationHint == ""
}

// =============================================================================
// Permission models — owned by toolreqs as the single authority
// =============================================================================

// Permission represents permission for an agent to use a specific tool within a tool set.
type Permission struct {
	UserID    string    `json:"user_id"`
	AgentID   string    `json:"agent_id"`
	ToolSetID string    `json:"tool_set_id"`
	ToolID    string    `json:"tool_id"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RequirementConfig parses the Config JSON into a typed [ToolRequirementConfig].
func (p Permission) RequirementConfig() ToolRequirementConfig {
	return ParseToolRequirementConfig(p.Config)
}

// ToolSetPermission represents an agent's access permission to a tool set.
type ToolSetPermission struct {
	UserID    string    `json:"user_id"`
	AgentID   string    `json:"agent_id"`
	ToolSetID string    `json:"tool_set_id"`
	Enabled   bool      `json:"enabled"`
	Config    string    `json:"config,omitempty"` // JSON string for tool set-specific config
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RequirementConfig parses the Config JSON into a typed [ToolRequirementConfig].
func (p ToolSetPermission) RequirementConfig() ToolRequirementConfig {
	return ParseToolRequirementConfig(p.Config)
}
