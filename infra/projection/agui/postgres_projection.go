package agui

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/event"
	"school-with-ai/foundation/logger"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/jmoiron/sqlx"
)

// PostgresAguiProjection is a PostgreSQL-backed implementation of AguiProjection.
type PostgresAguiProjection struct {
	log *logger.Logger
	db  *sqlx.DB
}

// NewPostgresAguiProjection creates a new PostgreSQL AGUI projection.
func NewPostgresAguiProjection(log *logger.Logger, db *sqlx.DB) *PostgresAguiProjection {
	return &PostgresAguiProjection{
		log: log,
		db:  db,
	}
}

// RegisterHandlers registers database update handlers to the provided router.
func (p *PostgresAguiProjection) RegisterHandlers(router interface {
	Register(eventPrototype any, handler event.HandlerFunc)
},
) {
	router.Register(&domain.ThreadCreated{}, func(ctx context.Context, e core.Event) error {
		return p.applyThreadCreated(ctx, *e.(*domain.ThreadCreated))
	})
	router.Register(&domain.MessageAdded{}, func(ctx context.Context, e core.Event) error {
		return p.applyMessageAdded(ctx, *e.(*domain.MessageAdded))
	})
	router.Register(&domain.ThreadTitleUpdatedEvent{}, func(ctx context.Context, e core.Event) error {
		return p.applyThreadTitleChanged(ctx, *e.(*domain.ThreadTitleUpdatedEvent))
	})
	router.Register(&domain.ThreadDeleted{}, func(ctx context.Context, e core.Event) error {
		return p.applyThreadDeleted(ctx, *e.(*domain.ThreadDeleted))
	})
}

// applyThreadCreated creates a new thread record.
func (p *PostgresAguiProjection) applyThreadCreated(ctx context.Context, event domain.ThreadCreated) error {
	query := `
		INSERT INTO agui_threads (thread_id, user_id, title, created_at, updated_at, is_deleted, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (thread_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, query,
		event.ThreadID,
		event.UserID,
		event.Title,
		event.Timestamp,
		event.Timestamp,
		false,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to insert thread: %w", err)
	}

	return nil
}

// applyMessageAdded adds a message to the thread.
func (p *PostgresAguiProjection) applyMessageAdded(ctx context.Context, event domain.MessageAdded) error {
	// First, get current message count for sequence number
	var messageCount int
	countQuery := `SELECT COUNT(*) FROM agui_messages WHERE thread_id = $1`
	if err := p.db.GetContext(ctx, &messageCount, countQuery, event.ThreadID); err != nil {
		return fmt.Errorf("failed to get message count: %w", err)
	}

	// Marshal metadata to JSONB
	var metadataJSON []byte
	if event.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Marshal tool calls to JSONB
	var toolCallsJSON []byte
	if len(event.ToolCalls) > 0 {
		var err error
		toolCallsJSON, err = json.Marshal(event.ToolCalls)
		if err != nil {
			return fmt.Errorf("failed to marshal tool calls: %w", err)
		}
	}

	// Insert message
	insertQuery := `
		INSERT INTO agui_messages (message_id, thread_id, role, content, tool_calls, tool_call_id, agent_name, metadata, created_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (message_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, insertQuery,
		event.MessageID,
		event.ThreadID,
		event.Role,
		event.Content,
		toolCallsJSON,
		event.ToolCallID,
		event.AgentName,
		metadataJSON,
		event.Timestamp,
		messageCount, // 0-indexed sequence
	)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Update thread's updated_at timestamp
	updateQuery := `
		UPDATE agui_threads 
		SET updated_at = $1, version = version + 1
		WHERE thread_id = $2
	`

	_, err = p.db.ExecContext(ctx, updateQuery, event.Timestamp, event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to update thread timestamp: %w", err)
	}

	return nil
}

// applyThreadTitleChanged updates the thread title.
func (p *PostgresAguiProjection) applyThreadTitleChanged(ctx context.Context, event domain.ThreadTitleUpdatedEvent) error {
	query := `
		UPDATE agui_threads
		SET title = $1, updated_at = $2, version = version + 1
		WHERE thread_id = $3
	`

	_, err := p.db.ExecContext(ctx, query, event.Title, time.Now(), event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to update thread title: %w", err)
	}

	return nil
}

// applyThreadDeleted marks a thread as deleted (messages cascade delete via FK).
func (p *PostgresAguiProjection) applyThreadDeleted(ctx context.Context, event domain.ThreadDeleted) error {
	query := `
		UPDATE agui_threads
		SET is_deleted = true, updated_at = $1, version = version + 1
		WHERE thread_id = $2
	`

	_, err := p.db.ExecContext(ctx, query, time.Now(), event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to mark thread as deleted: %w", err)
	}

	return nil
}

// GetThreadState retrieves the complete UI state for a thread from PostgreSQL.
func (p *PostgresAguiProjection) GetThreadState(threadID string) (*AguiThreadState, bool) {
	ctx := context.Background()

	// Get thread record
	var thread struct {
		ThreadID  string    `db:"thread_id"`
		UserID    string    `db:"user_id"`
		Title     string    `db:"title"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
		IsDeleted bool      `db:"is_deleted"`
		Version   int64     `db:"version"`
	}

	threadQuery := `
		SELECT thread_id, user_id, title, created_at, updated_at, is_deleted, version
		FROM agui_threads
		WHERE thread_id = $1
	`

	err := p.db.GetContext(ctx, &thread, threadQuery, threadID)
	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		p.log.Error(ctx, "failed to get thread", "threadID", threadID, "ERROR", err)
		return nil, false
	}

	// Don't return deleted threads
	if thread.IsDeleted {
		return nil, false
	}

	// Get messages
	var messages []struct {
		MessageID   string           `db:"message_id"`
		Role        string           `db:"role"`
		Content     string           `db:"content"`
		ToolCalls   *json.RawMessage `db:"tool_calls"`
		ToolCallID  *string          `db:"tool_call_id"`
		AgentName   *string          `db:"agent_name"`
		Metadata    *json.RawMessage `db:"metadata"` // Pointer to handle NULL
		CreatedAt   time.Time        `db:"created_at"`
		SequenceNum int              `db:"sequence_num"`
	}

	messagesQuery := `
		SELECT message_id, role, content, tool_calls, tool_call_id, agent_name, metadata, created_at, sequence_num
		FROM agui_messages
		WHERE thread_id = $1
		ORDER BY sequence_num ASC
	`

	err = p.db.SelectContext(ctx, &messages, messagesQuery, threadID)
	if err != nil && err != sql.ErrNoRows {
		p.log.Error(ctx, "failed to get messages", "threadID", threadID, "ERROR", err)
		return nil, false
	}

	// Convert database messages to AGUI SDK format
	aguiMessages := make([]aguievents.Message, 0, len(messages))
	for _, msg := range messages {
		content := msg.Content
		aguimsit := aguievents.Message{
			ID:      msg.MessageID,
			Role:    msg.Role,
			Content: &content,
		}

		if msg.ToolCallID != nil {
			aguimsit.ToolCallID = msg.ToolCallID
		}

		if msg.AgentName != nil {
			aguimsit.Name = msg.AgentName
		}

		if msg.ToolCalls != nil {
			var coreToolCalls []core.ToolCall
			if err := json.Unmarshal([]byte(*msg.ToolCalls), &coreToolCalls); err == nil {
				aguiToolCalls := make([]aguievents.ToolCall, 0, len(coreToolCalls))
				for _, tc := range coreToolCalls {
					aguiToolCalls = append(aguiToolCalls, aguievents.ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					})
				}
				aguimsit.ToolCalls = aguiToolCalls
			}
		}

		aguiMessages = append(aguiMessages, aguimsit)
	}

	state := &AguiThreadState{
		ThreadID:      thread.ThreadID,
		Title:         thread.Title,
		Messages:      aguiMessages,
		ToolCalls:     []AguiToolCall{},
		AgentActivity: make(map[string]*AguiAgentActivity),
		CreatedAt:     thread.CreatedAt,
		UpdatedAt:     thread.UpdatedAt,
		Version:       thread.Version,
		IsDeleted:     thread.IsDeleted,
	}

	return state, true
}


// GetMessages retrieves all UI-formatted messages for a thread.
func (p *PostgresAguiProjection) GetMessages(threadID string) []aguievents.Message {
	state, exists := p.GetThreadState(threadID)
	if !exists {
		return []aguievents.Message{}
	}
	return state.Messages
}

// ListThreads returns paginated threads for a user.
func (p *PostgresAguiProjection) ListThreads(req ThreadListRequest) (*ThreadListResponse, error) {
	ctx := context.Background()

	// Apply defaults and limits
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Fetch one extra to determine if there are more pages
	fetchLimit := limit + 1

	var threads []struct {
		ThreadID  string    `db:"thread_id"`
		Title     *string   `db:"title"`
		UpdatedAt time.Time `db:"updated_at"`
	}

	var err error
	if req.Cursor == "" {
		// First page - no cursor
		query := `
			SELECT thread_id, title, updated_at
			FROM agui_threads
			WHERE user_id = $1 AND is_deleted = false
			ORDER BY updated_at DESC
			LIMIT $2
		`
		err = p.db.SelectContext(ctx, &threads, query, req.UserID, fetchLimit)
	} else {
		// Subsequent pages - use cursor
		cursorTime, parseErr := time.Parse(time.RFC3339Nano, req.Cursor)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid cursor format: %w", parseErr)
		}

		query := `
			SELECT thread_id, title, updated_at
			FROM agui_threads
			WHERE user_id = $1 AND is_deleted = false AND updated_at < $2
			ORDER BY updated_at DESC
			LIMIT $3
		`
		err = p.db.SelectContext(ctx, &threads, query, req.UserID, cursorTime, fetchLimit)
	}

	if err != nil {
		p.log.Error(ctx, "failed to list threads", "userID", req.UserID, "ERROR", err)
		return nil, fmt.Errorf("failed to list threads: %w", err)
	}

	// Determine if there are more pages
	hasMore := len(threads) > limit
	if hasMore {
		threads = threads[:limit] // Trim the extra item
	}

	// Convert to response format
	result := &ThreadListResponse{
		Threads: make([]ThreadSummary, 0, len(threads)),
		HasMore: hasMore,
	}

	for _, t := range threads {
		title := ""
		if t.Title != nil {
			title = *t.Title
		}
		result.Threads = append(result.Threads, ThreadSummary{
			ID:        t.ThreadID,
			Title:     title,
			UpdatedAt: t.UpdatedAt,
		})
	}

	// Set next cursor if there are more pages
	if hasMore && len(result.Threads) > 0 {
		lastThread := result.Threads[len(result.Threads)-1]
		result.NextCursor = lastThread.UpdatedAt.Format(time.RFC3339Nano)
	}

	return result, nil
}

// Verify PostgresAguiProjection implements AguiProjection.
var _ AguiProjection = (*PostgresAguiProjection)(nil)
