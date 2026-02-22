package llmcontext

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/event"
	"school-with-ai/foundation/logger"

	"github.com/jmoiron/sqlx"
)

// PostgresLLMContextProjection is a PostgreSQL-backed implementation for LLM context.
// It maintains materialized thread and message state in PostgreSQL tables that are
// incrementally updated via the Apply() method when events are received.
//
// The web layer only uses domain-agnostic methods (Get, Messages, All).
type PostgresLLMContextProjection struct {
	log *logger.Logger
	db  *sqlx.DB
}

// NewPostgresLLMContextProjection creates a new PostgreSQL LLM context projection.
func NewPostgresLLMContextProjection(log *logger.Logger, db *sqlx.DB) *PostgresLLMContextProjection {
	return &PostgresLLMContextProjection{
		log: log,
		db:  db,
	}
}

// RegisterHandlers registers database update handlers to the provided router.
func (p *PostgresLLMContextProjection) RegisterHandlers(router interface {
	Register(eventPrototype any, handler event.HandlerFunc)
},
) {
	router.Register(&domain.ThreadCreated{}, func(ctx context.Context, e domain.Event) error {
		return p.applyThreadCreated(ctx, e.(*domain.ThreadCreated))
	})
	router.Register(&domain.UserMessageAdded{}, func(ctx context.Context, e domain.Event) error {
		return p.applyUserMessageAdded(ctx, e.(*domain.UserMessageAdded))
	})
	router.Register(&domain.AssistantMessageAdded{}, func(ctx context.Context, e domain.Event) error {
		return p.applyAssistantMessageAdded(ctx, e.(*domain.AssistantMessageAdded))
	})
	router.Register(&domain.ToolResultAdded{}, func(ctx context.Context, e domain.Event) error {
		return p.applyToolResultAdded(ctx, e.(*domain.ToolResultAdded))
	})
	router.Register(&domain.ThreadTitleUpdated{}, func(ctx context.Context, e domain.Event) error {
		return p.applyThreadTitleChanged(ctx, e.(*domain.ThreadTitleUpdated))
	})
	router.Register(&domain.ThreadDeleted{}, func(ctx context.Context, e domain.Event) error {
		return p.applyThreadDeleted(ctx, e.(*domain.ThreadDeleted))
	})
}

// applyThreadCreated creates a new thread record.
func (p *PostgresLLMContextProjection) applyThreadCreated(ctx context.Context, event *domain.ThreadCreated) error {
	query := `
		INSERT INTO llm_threads (thread_id, user_id, title, created_at, updated_at, is_deleted)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (thread_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, query,
		event.ThreadID,
		event.UserID,
		event.Title,
		event.Timestamp,
		event.Timestamp,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to insert thread: %w", err)
	}

	return nil
}

// applyUserMessageAdded adds a user message to the thread.
func (p *PostgresLLMContextProjection) applyUserMessageAdded(ctx context.Context, event *domain.UserMessageAdded) error {
	// Use atomic subquery for sequence number to prevent race conditions.
	insertQuery := `
		INSERT INTO llm_messages (message_id, thread_id, role, content, created_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, (SELECT COUNT(*) FROM llm_messages WHERE thread_id = $2))
		ON CONFLICT (message_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, insertQuery,
		event.MessageID,
		event.ThreadID,
		"user",
		event.Content,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Update thread's updated_at timestamp
	updateQuery := `
		UPDATE llm_threads 
		SET updated_at = $1
		WHERE thread_id = $2
	`

	_, err = p.db.ExecContext(ctx, updateQuery, event.Timestamp, event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to update thread timestamp: %w", err)
	}

	return nil
}

// applyAssistantMessageAdded adds an assistant message to the thread.
func (p *PostgresLLMContextProjection) applyAssistantMessageAdded(ctx context.Context, event *domain.AssistantMessageAdded) error {
	// Marshal tool calls to JSONB
	var toolCallsJSON []byte
	if len(event.ToolCalls) > 0 {
		var err error
		toolCallsJSON, err = json.Marshal(event.ToolCalls)
		if err != nil {
			return fmt.Errorf("failed to marshal tool calls: %w", err)
		}
	}

	// Use atomic subquery for sequence number to prevent race conditions.
	insertQuery := `
		INSERT INTO llm_messages (message_id, thread_id, role, content, tool_calls, agent_name, created_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT COUNT(*) FROM llm_messages WHERE thread_id = $2))
		ON CONFLICT (message_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, insertQuery,
		event.MessageID,
		event.ThreadID,
		"assistant",
		event.Content,
		toolCallsJSON,
		event.AgentName,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Update thread's updated_at timestamp
	updateQuery := `
		UPDATE llm_threads 
		SET updated_at = $1
		WHERE thread_id = $2
	`

	_, err = p.db.ExecContext(ctx, updateQuery, event.Timestamp, event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to update thread timestamp: %w", err)
	}

	return nil
}

// applyToolResultAdded adds a tool result message to the thread.
func (p *PostgresLLMContextProjection) applyToolResultAdded(ctx context.Context, event *domain.ToolResultAdded) error {
	// Use atomic subquery for sequence number to prevent race conditions.
	insertQuery := `
		INSERT INTO llm_messages (message_id, thread_id, role, content, tool_call_id, agent_name, created_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT COUNT(*) FROM llm_messages WHERE thread_id = $2))
		ON CONFLICT (message_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, insertQuery,
		event.MessageID,
		event.ThreadID,
		"tool",
		event.Result,
		event.ToolCallID,
		event.AgentName,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert message: %w", err)
	}

	// Update thread's updated_at timestamp
	updateQuery := `
		UPDATE llm_threads 
		SET updated_at = $1
		WHERE thread_id = $2
	`

	_, err = p.db.ExecContext(ctx, updateQuery, event.Timestamp, event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to update thread timestamp: %w", err)
	}

	return nil
}

// applyThreadTitleChanged updates the thread title.
func (p *PostgresLLMContextProjection) applyThreadTitleChanged(ctx context.Context, event *domain.ThreadTitleUpdated) error {
	query := `
		UPDATE llm_threads
		SET title = $1, updated_at = $2
		WHERE thread_id = $3
	`

	_, err := p.db.ExecContext(ctx, query, event.Title, time.Now(), event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to update thread title: %w", err)
	}

	return nil
}

// applyThreadDeleted marks a thread as deleted (messages cascade delete via FK).
func (p *PostgresLLMContextProjection) applyThreadDeleted(ctx context.Context, event *domain.ThreadDeleted) error {
	query := `
		UPDATE llm_threads
		SET is_deleted = true, updated_at = $1
		WHERE thread_id = $2
	`

	_, err := p.db.ExecContext(ctx, query, time.Now(), event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to mark thread as deleted: %w", err)
	}

	return nil
}

// Get retrieves a thread from PostgreSQL.
func (p *PostgresLLMContextProjection) Get(threadID string) (*domain.Thread, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get thread record
	var thread struct {
		ThreadID  string    `db:"thread_id"`
		UserID    string    `db:"user_id"`
		Title     string    `db:"title"`
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
		IsDeleted bool      `db:"is_deleted"`
	}

	threadQuery := `
		SELECT thread_id, user_id, title, created_at, updated_at, is_deleted
		FROM llm_threads
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
	messages, err := p.getMessages(ctx, threadID)
	if err != nil {
		p.log.Error(ctx, "failed to get messages", "threadID", threadID, "ERROR", err)
		return nil, false
	}

	return &domain.Thread{
		ID:        thread.ThreadID,
		Title:     thread.Title,
		Messages:  messages,
		CreatedAt: thread.CreatedAt,
		UpdatedAt: thread.UpdatedAt,
	}, true
}

// getMessages retrieves messages for a thread.
func (p *PostgresLLMContextProjection) getMessages(ctx context.Context, threadID string) ([]domain.Message, error) {
	var dbMessages []struct {
		MessageID  string           `db:"message_id"`
		Role       string           `db:"role"`
		Content    string           `db:"content"`
		ToolCalls  *json.RawMessage `db:"tool_calls"`
		ToolCallID string           `db:"tool_call_id"`
		AgentName  string           `db:"agent_name"`
		Metadata   *json.RawMessage `db:"metadata"`
		CreatedAt  time.Time        `db:"created_at"`
	}

	messagesQuery := `
		SELECT message_id, role, content, tool_calls, tool_call_id, agent_name, metadata, created_at
		FROM llm_messages
		WHERE thread_id = $1
		ORDER BY sequence_num ASC
	`

	err := p.db.SelectContext(ctx, &dbMessages, messagesQuery, threadID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	messages := make([]domain.Message, 0, len(dbMessages))
	for _, msg := range dbMessages {
		// Unmarshal tool calls
		var toolCalls []domain.ToolCall
		if msg.ToolCalls != nil {
			if err := json.Unmarshal(*msg.ToolCalls, &toolCalls); err != nil {
				p.log.Error(ctx, "failed to unmarshal tool calls", "messageID", msg.MessageID, "ERROR", err)
			}
		}

		// Unmarshal metadata
		var metadata map[string]any
		if msg.Metadata != nil {
			if err := json.Unmarshal(*msg.Metadata, &metadata); err != nil {
				p.log.Error(ctx, "failed to unmarshal metadata", "messageID", msg.MessageID, "ERROR", err)
			}
		}

		messages = append(messages, domain.Message{
			ID:         msg.MessageID,
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  toolCalls,
			ToolCallID: msg.ToolCallID,
			AgentName:  msg.AgentName,
			Metadata:   metadata,
		})
	}

	return messages, nil
}

// Messages retrieves all messages from a thread.
func (p *PostgresLLMContextProjection) Messages(threadID string) []domain.Message {
	if thread, exists := p.Get(threadID); exists {
		return thread.Messages
	}
	return []domain.Message{}
}

// All retrieves all non-deleted threads (capped to prevent unbounded queries).
func (p *PostgresLLMContextProjection) All() []domain.Thread {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var threadIDs []string
	query := `
		SELECT thread_id
		FROM llm_threads
		WHERE is_deleted = false
		ORDER BY updated_at DESC
		LIMIT 500
	`

	err := p.db.SelectContext(ctx, &threadIDs, query)
	if err != nil {
		p.log.Error(ctx, "failed to list threads", "ERROR", err)
		return []domain.Thread{}
	}

	threads := make([]domain.Thread, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		if thread, exists := p.Get(threadID); exists {
			threads = append(threads, *thread)
		}
	}

	return threads
}
