package statistics

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/event"

	"github.com/jmoiron/sqlx"
)

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
func (p *PostgresProjection) RegisterHandlers(router interface {
	Register(eventPrototype any, handler event.HandlerFunc)
},
) {
	router.Register(&domain.ToolSuccess{}, func(ctx context.Context, e domain.Event) error {
		return p.HandleEvent(ctx, e)
	})
	router.Register(&domain.ToolError{}, func(ctx context.Context, e domain.Event) error {
		return p.HandleEvent(ctx, e)
	})
	router.Register(&domain.AssistantMessageCompleted{}, func(ctx context.Context, e domain.Event) error {
		return p.HandleEvent(ctx, e)
	})
	router.Register(&domain.RunError{}, func(ctx context.Context, e domain.Event) error {
		return p.HandleEvent(ctx, e)
	})
}

// HandleEvent processes domain events to update statistics in the database.
func (p *PostgresProjection) HandleEvent(ctx context.Context, event domain.Event) error {
	switch e := event.(type) {
	case *domain.ToolSuccess:
		return p.updateToolSuccessStats(ctx, e)
	case *domain.ToolError:
		return p.updateToolErrorStats(ctx, e)
	case *domain.AssistantMessageCompleted:
		return p.updateMessageStats(ctx, e)
	case *domain.RunError:
		// We'll need to figure out which agent was active for RunError.
		// For now, let's just track it as a global or see if we can get agent from context.
		return p.updateErrorStats(ctx, e)
	}
	return nil
}

func (p *PostgresProjection) updateToolSuccessStats(ctx context.Context, e *domain.ToolSuccess) error {
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

func (p *PostgresProjection) updateToolErrorStats(ctx context.Context, e *domain.ToolError) error {
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

func (p *PostgresProjection) updateMessageStats(ctx context.Context, e *domain.AssistantMessageCompleted) error {
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

func (p *PostgresProjection) updateErrorStats(ctx context.Context, e *domain.RunError) error {
	// Attempt to get agent name from context if available, otherwise use 'unknown'
	agentName := "unknown"
	query := `
		INSERT INTO analytics_stats (agent_name, category, name, total_count, failures, last_failure, updated_at)
		VALUES ($1, 'error', 'total', 1, 1, $2, $3)
		ON CONFLICT (agent_name, category, name) DO UPDATE SET
			total_count = analytics_stats.total_count + 1,
			failures = analytics_stats.failures + 1,
			last_failure = $2,
			updated_at = $3
	`
	_, err := p.db.ExecContext(ctx, query, agentName, e.Timestamp, time.Now())
	return err
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
