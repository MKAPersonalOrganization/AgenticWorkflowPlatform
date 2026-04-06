package statistics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aicompass/foundation/event"
	es "aicompass/foundation/eventsourcing"

	"github.com/jmoiron/sqlx"
)

// ---------------------------------------------------------------------------
// Local event DTOs — mirror the fields the projection actually reads from
// the chat domain events.  By defining them here the analytics module does
// not import modules/chat/thread at compile-time.  The router's name-based
// dispatch delivers the concrete event; we JSON-round-trip it into these DTOs
// to read the fields we need.
// ---------------------------------------------------------------------------

// toolResultEvent mirrors the fields of thread.ToolResultAdded used here.
type toolResultEvent struct {
	ThreadID  string    `json:"threadId"`
	ToolName  string    `json:"toolName"`
	IsError   bool      `json:"isError"`
	AgentName string    `json:"agentName"`
	Timestamp time.Time `json:"timestamp"`
}

// assistantMessageEvent mirrors thread.AssistantMessageCompleted.
type assistantMessageEvent struct {
	AgentName string    `json:"agentName"`
	Timestamp time.Time `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Projection
// ---------------------------------------------------------------------------

// PostgresProjection projects tool statistics into PostgreSQL.
type PostgresProjection struct {
	db *sqlx.DB
}

// NewPostgresProjection creates a new PostgreSQL statistics projection.
func NewPostgresProjection(db *sqlx.DB) *PostgresProjection {
	return &PostgresProjection{
		db: db,
	}
}

// RegisterHandlers registers database update handlers to the provided router.
// Uses name-based registration so the analytics module does not depend on the
// concrete event types defined in modules/chat/thread.
func (p *PostgresProjection) RegisterHandlers(router interface {
	RegisterByName(eventTypeName string, handler event.HandlerFunc)
},
) {
	router.RegisterByName("ToolResultAdded", func(ctx context.Context, e es.Event) error {
		return p.handleToolResult(ctx, e)
	})
	router.RegisterByName("AssistantMessageCompleted", func(ctx context.Context, e es.Event) error {
		return p.handleAssistantMessage(ctx, e)
	})
}

// eventToDTO JSON-round-trips an es.Event into the target DTO.
func eventToDTO[T any](evt es.Event) (T, error) {
	var dto T
	raw, err := json.Marshal(evt)
	if err != nil {
		return dto, fmt.Errorf("marshal event: %w", err)
	}
	if err := json.Unmarshal(raw, &dto); err != nil {
		return dto, fmt.Errorf("unmarshal event to DTO: %w", err)
	}
	return dto, nil
}

func (p *PostgresProjection) handleToolResult(ctx context.Context, evt es.Event) error {
	e, err := eventToDTO[toolResultEvent](evt)
	if err != nil {
		return err
	}

	if e.IsError {
		if err := p.updateToolErrorStats(ctx, &e); err != nil {
			return err
		}
	} else {
		if err := p.updateToolSuccessStats(ctx, &e); err != nil {
			return err
		}
	}
	return p.recordToolEvent(ctx, &e)
}

func (p *PostgresProjection) handleAssistantMessage(ctx context.Context, evt es.Event) error {
	e, err := eventToDTO[assistantMessageEvent](evt)
	if err != nil {
		return err
	}
	return p.updateMessageStats(ctx, &e)
}

func (p *PostgresProjection) updateToolSuccessStats(ctx context.Context, e *toolResultEvent) error {
	query := `
		INSERT INTO analytics_stats (agent_name, category, name, total_count, successes, failures, last_success, updated_at)
		VALUES ($1, 'tool', $2, 1, 1, 0, $3, $4)
		ON CONFLICT (agent_name, category, name) DO UPDATE SET
			total_count = analytics_stats.total_count + 1,
			successes = analytics_stats.successes + 1,
			last_success = $3,
			updated_at = $4
	`

	_, err := p.db.ExecContext(ctx, query,
		e.AgentName,
		e.ToolName,
		e.Timestamp,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to update tool success stats for %s (agent: %s): %w", e.ToolName, e.AgentName, err)
	}

	return nil
}

func (p *PostgresProjection) updateToolErrorStats(ctx context.Context, e *toolResultEvent) error {
	query := `
		INSERT INTO analytics_stats (agent_name, category, name, total_count, successes, failures, last_failure, updated_at)
		VALUES ($1, 'tool', $2, 1, 0, 1, $3, $4)
		ON CONFLICT (agent_name, category, name) DO UPDATE SET
			total_count = analytics_stats.total_count + 1,
			failures = analytics_stats.failures + 1,
			last_failure = $3,
			updated_at = $4
	`

	_, err := p.db.ExecContext(ctx, query,
		e.AgentName,
		e.ToolName,
		e.Timestamp,
		time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to update tool error stats for %s (agent: %s): %w", e.ToolName, e.AgentName, err)
	}

	return nil
}

func (p *PostgresProjection) updateMessageStats(ctx context.Context, e *assistantMessageEvent) error {
	query := `
		INSERT INTO analytics_stats (agent_name, category, name, total_count, successes, updated_at)
		VALUES ($1, 'message', 'total', 1, 1, $2)
		ON CONFLICT (agent_name, category, name) DO UPDATE SET
			total_count = analytics_stats.total_count + 1,
			successes = analytics_stats.successes + 1,
			updated_at = $2
	`
	_, err := p.db.ExecContext(ctx, query, e.AgentName, time.Now())
	return err
}

// recordToolEvent inserts an individual tool event for time-series analysis.
func (p *PostgresProjection) recordToolEvent(ctx context.Context, e *toolResultEvent) error {
	query := `
		INSERT INTO analytics_events (agent_name, tool_name, is_error, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := p.db.ExecContext(ctx, query,
		e.AgentName,
		e.ToolName,
		e.IsError,
		e.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to record tool event for %s (agent: %s): %w", e.ToolName, e.AgentName, err)
	}
	return nil
}

// GetStats retrieves statistics, optionally filtered by agent.
func (p *PostgresProjection) GetStats(ctx context.Context, agentName string) ([]StatRow, error) {
	var stats []StatRow
	query := `
		SELECT agent_name, category, name, total_count, successes, failures, last_failure, last_success, updated_at
		FROM analytics_stats
	`
	var args []any
	if agentName != "" {
		query += " WHERE agent_name = $1 "
		args = append(args, agentName)
	}
	query += " ORDER BY category ASC, total_count DESC"

	if err := p.db.SelectContext(ctx, &stats, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return stats, nil
}

// GetAgents retrieves a list of all unique agent names in the stats.
func (p *PostgresProjection) GetAgents(ctx context.Context) ([]string, error) {
	var agents []string
	query := "SELECT DISTINCT agent_name FROM analytics_stats ORDER BY agent_name ASC"
	if err := p.db.SelectContext(ctx, &agents, query); err != nil {
		return nil, fmt.Errorf("failed to get agents: %w", err)
	}
	return agents, nil
}

// GetToolTimeSeries retrieves time-series data for tool calls, grouped by hourly buckets.
// Optionally filtered by agent and/or tool name.
func (p *PostgresProjection) GetToolTimeSeries(ctx context.Context, agentFilter, toolFilter string) ([]TimeSeriesPoint, error) {
	var points []TimeSeriesPoint
	query := `
		SELECT
			date_trunc('hour', created_at) AS bucket,
			COUNT(*) AS total_calls,
			SUM(CASE WHEN is_error THEN 1 ELSE 0 END) AS errors,
			SUM(CASE WHEN NOT is_error THEN 1 ELSE 0 END) AS successes,
			CASE WHEN COUNT(*) > 0 THEN ROUND(SUM(CASE WHEN is_error THEN 1 ELSE 0 END)::numeric / COUNT(*)::numeric * 100, 2) ELSE 0 END AS error_rate,
			AVG(latency_ms)::double precision AS avg_latency
		FROM analytics_events
		WHERE created_at >= NOW() - INTERVAL '7 days'
	`
	var args []any
	argIdx := 1

	if agentFilter != "" {
		query += fmt.Sprintf(" AND agent_name = $%d", argIdx)
		args = append(args, agentFilter)
		argIdx++
	}
	if toolFilter != "" {
		query += fmt.Sprintf(" AND tool_name = $%d", argIdx)
		args = append(args, toolFilter)
		argIdx++
	}

	query += " GROUP BY bucket ORDER BY bucket ASC"

	if err := p.db.SelectContext(ctx, &points, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get tool time series: %w", err)
	}
	return points, nil
}

// GetToolList retrieves a summary of all tools, optionally filtered by agent.
func (p *PostgresProjection) GetToolList(ctx context.Context, agentFilter string) ([]ToolSummary, error) {
	var tools []ToolSummary
	query := `
		SELECT
			tool_name,
			COUNT(*) AS total_calls,
			SUM(CASE WHEN is_error THEN 1 ELSE 0 END) AS errors,
			SUM(CASE WHEN NOT is_error THEN 1 ELSE 0 END) AS successes,
			CASE WHEN COUNT(*) > 0 THEN ROUND(SUM(CASE WHEN NOT is_error THEN 1 ELSE 0 END)::numeric / COUNT(*)::numeric * 100, 2) ELSE 0 END AS success_rate,
			AVG(latency_ms)::double precision AS avg_latency,
			MAX(created_at) AS last_call
		FROM analytics_events
	`
	var args []any
	if agentFilter != "" {
		query += " WHERE agent_name = $1"
		args = append(args, agentFilter)
	}

	query += " GROUP BY tool_name ORDER BY total_calls DESC"

	if err := p.db.SelectContext(ctx, &tools, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get tool list: %w", err)
	}
	return tools, nil
}

// GetToolDetail retrieves time-series data for a specific tool.
func (p *PostgresProjection) GetToolDetail(ctx context.Context, toolName, agentFilter string) ([]TimeSeriesPoint, error) {
	return p.GetToolTimeSeries(ctx, agentFilter, toolName)
}
