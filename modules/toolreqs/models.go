package toolreqs

import "time"

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
