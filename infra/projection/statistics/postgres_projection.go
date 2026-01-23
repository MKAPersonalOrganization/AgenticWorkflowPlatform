package statistics

import (
	"context"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/event"
	"time"

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
}) {
	router.Register(&domain.ToolCallExecutedEvent{}, func(ctx context.Context, e core.Event) error {
		return p.HandleEvent(ctx, e)
	})
}

// HandleEvent processes domain events to update statistics in the database.
func (p *PostgresProjection) HandleEvent(ctx context.Context, event core.Event) error {
	switch e := event.(type) {
	case *domain.ToolCallExecutedEvent:
		return p.updateToolStats(ctx, e)
	}
	return nil
}

func (p *PostgresProjection) updateToolStats(ctx context.Context, e *domain.ToolCallExecutedEvent) error {
	// Construct upsert query
	// We increment total_calls, and conditionally increment successes/failures and update timestamps.
	// Since standard SQL doesn't have a clean "upsert with conditional logic driven by application logic" in one go 
	// without passing params, we'll pass the increments as params.

	var (
		incSuccess = 0
		incFailure = 0
		lastSucc   *time.Time
		lastFail   *time.Time
	)

	if e.IsError {
		incFailure = 1
		lastFail = &e.Timestamp
	} else {
		incSuccess = 1
		lastSucc = &e.Timestamp
	}

	query := `
		INSERT INTO tool_stats (tool_name, agent_name, total_calls, successes, failures, last_failure, last_success, updated_at)
		VALUES ($1, $2, 1, $3, $4, $5, $6, $7)
		ON CONFLICT (tool_name, agent_name) DO UPDATE SET
			total_calls = tool_stats.total_calls + 1,
			successes = tool_stats.successes + $3,
			failures = tool_stats.failures + $4,
			last_failure = COALESCE($5, tool_stats.last_failure),
			last_success = COALESCE($6, tool_stats.last_success),
			updated_at = $7
	`

	_, err := p.db.ExecContext(ctx, query,
		e.ToolName,
		e.AgentName,
		incSuccess,
		incFailure,
		lastFail,
		lastSucc,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to update tool stats for %s (agent: %s): %w", e.ToolName, e.AgentName, err)
	}

	return nil
}

// GetStats retrieves all tool statistics.
func (p *PostgresProjection) GetStats(ctx context.Context) ([]ToolStats, error) {
	var stats []ToolStats
	query := `
		SELECT tool_name, agent_name, total_calls, successes, failures, last_failure, last_success
		FROM tool_stats
		ORDER BY failures DESC, total_calls DESC
	`
	if err := p.db.SelectContext(ctx, &stats, query); err != nil {
		return nil, fmt.Errorf("failed to get tool stats: %w", err)
	}
	return stats, nil
}
