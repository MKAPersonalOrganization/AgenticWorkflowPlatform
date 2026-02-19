package statistics

import "time"

// StatRow represents a row in the unified analytics statistics table.
type StatRow struct {
	AgentName   string     `json:"agentName" db:"agent_name"`
	Category    string     `json:"category" db:"category"` // 'tool', 'message', 'error'
	Name        string     `json:"name" db:"name"`         // tool name or metric name
	TotalCount  int        `json:"totalCount" db:"total_count"`
	Successes   int        `json:"successes" db:"successes"`
	Failures    int        `json:"failures" db:"failures"`
	LastFailure *time.Time `json:"lastFailure,omitempty" db:"last_failure"`
	LastSuccess *time.Time `json:"lastSuccess,omitempty" db:"last_success"`
	UpdatedAt   time.Time  `json:"updatedAt" db:"updated_at"`
}

// TimeSeriesPoint represents an aggregated data point for time-series charts.
type TimeSeriesPoint struct {
	Bucket     time.Time `json:"bucket" db:"bucket"`
	TotalCalls int       `json:"totalCalls" db:"total_calls"`
	Errors     int       `json:"errors" db:"errors"`
	Successes  int       `json:"successes" db:"successes"`
	ErrorRate  float64   `json:"errorRate" db:"error_rate"`
	AvgLatency *float64  `json:"avgLatency,omitempty" db:"avg_latency"`
}

// ToolSummary provides an aggregated summary for a single tool.
type ToolSummary struct {
	ToolName    string     `json:"toolName" db:"tool_name"`
	TotalCalls  int        `json:"totalCalls" db:"total_calls"`
	Errors      int        `json:"errors" db:"errors"`
	Successes   int        `json:"successes" db:"successes"`
	SuccessRate float64    `json:"successRate" db:"success_rate"`
	AvgLatency  *float64   `json:"avgLatency,omitempty" db:"avg_latency"`
	LastCall    *time.Time `json:"lastCall,omitempty" db:"last_call"`
}
