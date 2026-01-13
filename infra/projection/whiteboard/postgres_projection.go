package whiteboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"

	"github.com/jmoiron/sqlx"
)

// PostgresWhiteboardProjection is a PostgreSQL-backed implementation of WhiteboardStore.
// It maintains materialized whiteboard content in PostgreSQL tables that are incrementally updated
// via the Apply() method when events are received from the subscription.
type PostgresWhiteboardProjection struct {
	log *logger.Logger
	db  *sqlx.DB
}

// NewPostgresWhiteboardProjection creates a new PostgreSQL whiteboard projection.
func NewPostgresWhiteboardProjection(log *logger.Logger, db *sqlx.DB) *PostgresWhiteboardProjection {
	return &PostgresWhiteboardProjection{
		log: log,
		db:  db,
	}
}

// Name returns the projection name for logging and checkpointing.
func (p *PostgresWhiteboardProjection) Name() string {
	return "whiteboard"
}

// Apply handles an event and updates the PostgreSQL projection.
func (p *PostgresWhiteboardProjection) Apply(ctx context.Context, event core.Event) error {
	switch e := event.(type) {
	case domain.MessageAdded:
		return p.applyMessageAdded(ctx, e)
	case *domain.MessageAdded:
		return p.applyMessageAdded(ctx, *e)
	default:
		// Ignore unknown events
		return nil
	}
}

// applyMessageAdded processes MessageAdded events and updates whiteboard if write_whiteboard tool was called.
func (p *PostgresWhiteboardProjection) applyMessageAdded(ctx context.Context, event domain.MessageAdded) error {
	// Only process messages with tool calls
	if len(event.ToolCalls) == 0 {
		return nil
	}

	// Check for write_whiteboard tool call
	for _, tc := range event.ToolCalls {
		if tc.Function.Name == "write_whiteboard" {
			// Extract content from tool call arguments
			content := extractWhiteboardContent(tc.Function.Arguments)
			if content == "" {
				continue
			}

			// Upsert whiteboard content
			query := `
				INSERT INTO whiteboards (thread_id, content, updated_at, version)
				VALUES ($1, $2, $3, 0)
				ON CONFLICT (thread_id) 
				DO UPDATE SET content = $2, updated_at = $3, version = whiteboards.version + 1
			`

			_, err := p.db.ExecContext(ctx, query, event.ThreadID, content, event.Timestamp)
			if err != nil {
				return fmt.Errorf("failed to update whiteboard: %w", err)
			}

			p.log.Info(ctx, "whiteboard updated",
				"threadID", event.ThreadID,
				"contentLength", len(content))
		}
	}

	return nil
}

// Get retrieves the whiteboard content for a thread from PostgreSQL.
func (p *PostgresWhiteboardProjection) Get(threadID string) string {
	ctx := context.Background()

	var content string
	query := `SELECT content FROM whiteboards WHERE thread_id = $1`

	err := p.db.GetContext(ctx, &content, query, threadID)
	if err == sql.ErrNoRows {
		return ""
	}
	if err != nil {
		p.log.Error(ctx, "failed to get whiteboard", "threadID", threadID, "ERROR", err)
		return ""
	}

	return content
}

// Update is kept for interface compatibility but delegates to Apply.
func (p *PostgresWhiteboardProjection) Update(event any) error {
	ctx := context.Background()

	// Convert to core.Event if possible
	if coreEvent, ok := event.(core.Event); ok {
		return p.Apply(ctx, coreEvent)
	}

	return nil
}

// extractWhiteboardContent extracts the content field from tool call arguments JSON.
func extractWhiteboardContent(arguments string) string {
	var args struct {
		Content string `json:"content"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}

	return args.Content
}

// Verify PostgresWhiteboardProjection implements domain.WhiteboardStore.
var _ domain.WhiteboardStore = (*PostgresWhiteboardProjection)(nil)
