package statistics

import "time"

// ToolStats represents statistics for a specific tool.
type ToolStats struct {
	ToolName    string     `json:"toolName" db:"tool_name"`
	AgentName   string     `json:"agentName" db:"agent_name"`
	TotalCalls  int        `json:"totalCalls" db:"total_calls"`
	Successes   int        `json:"successes" db:"successes"`
	Failures    int        `json:"failures" db:"failures"`
	LastFailure *time.Time `json:"lastFailure,omitempty" db:"last_failure"`
	LastSuccess *time.Time `json:"lastSuccess,omitempty" db:"last_success"`
}
