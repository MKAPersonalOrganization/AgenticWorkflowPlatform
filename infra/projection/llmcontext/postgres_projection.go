package llmcontext

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"time"

	"github.com/jmoiron/sqlx"
)

// PostgresLLMContextProjection is a PostgreSQL-backed implementation for LLM context.
// It maintains materialized thread and message state in PostgreSQL tables that are
// incrementally updated via the Apply() method when events are received.
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

// Name returns the projection name for logging and checkpointing.
func (p *PostgresLLMContextProjection) Name() string {
	return "llm_context"
}

// Apply handles an event and updates the PostgreSQL projection.
func (p *PostgresLLMContextProjection) Apply(ctx context.Context, event core.Event) error {
	switch e := event.(type) {
	case domain.ThreadCreated:
		return p.applyThreadCreated(ctx, e)
	case domain.MessageAdded:
		return p.applyMessageAdded(ctx, e)
	case domain.ThreadTitleChanged:
		return p.applyThreadTitleChanged(ctx, e)
	case domain.ThreadDeleted:
		return p.applyThreadDeleted(ctx, e)
	default:
		// Ignore unknown events
		return nil
	}
}

// applyThreadCreated creates a new thread record.
func (p *PostgresLLMContextProjection) applyThreadCreated(ctx context.Context, event domain.ThreadCreated) error {
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

// applyMessageAdded adds a message to the thread.
func (p *PostgresLLMContextProjection) applyMessageAdded(ctx context.Context, event domain.MessageAdded) error {
	// First, get current message count for sequence number
	var messageCount int
	countQuery := `SELECT COUNT(*) FROM llm_messages WHERE thread_id = $1`
	if err := p.db.GetContext(ctx, &messageCount, countQuery, event.ThreadID); err != nil {
		return fmt.Errorf("failed to get message count: %w", err)
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

	// Marshal metadata to JSONB
	var metadataJSON []byte
	if event.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	// Insert message
	insertQuery := `
		INSERT INTO llm_messages (message_id, thread_id, role, content, tool_calls, tool_call_id, agent_name, metadata, created_at, sequence_num)
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
func (p *PostgresLLMContextProjection) applyThreadTitleChanged(ctx context.Context, event domain.ThreadTitleChanged) error {
	query := `
		UPDATE llm_threads
		SET title = $1, updated_at = $2
		WHERE thread_id = $3
	`

	_, err := p.db.ExecContext(ctx, query, event.NewTitle, time.Now(), event.ThreadID)
	if err != nil {
		return fmt.Errorf("failed to update thread title: %w", err)
	}

	return nil
}

// applyThreadDeleted marks a thread as deleted (messages cascade delete via FK).
func (p *PostgresLLMContextProjection) applyThreadDeleted(ctx context.Context, event domain.ThreadDeleted) error {
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
	ctx := context.Background()

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
		var toolCalls []core.ToolCall
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

// GetAgentThread retrieves thread as AgentThread for LLM processing.
func (p *PostgresLLMContextProjection) GetAgentThread(threadID string) (core.AgentThread, bool) {
	ctx := context.Background()

	// Get thread
	var thread struct {
		ThreadID  string `db:"thread_id"`
		IsDeleted bool   `db:"is_deleted"`
	}

	threadQuery := `SELECT thread_id, is_deleted FROM llm_threads WHERE thread_id = $1`
	err := p.db.GetContext(ctx, &thread, threadQuery, threadID)
	if err == sql.ErrNoRows || thread.IsDeleted {
		return nil, false
	}
	if err != nil {
		p.log.Error(ctx, "failed to get thread", "threadID", threadID, "ERROR", err)
		return nil, false
	}

	// Get messages
	var dbMessages []struct {
		Role       string           `db:"role"`
		Content    string           `db:"content"`
		ToolCalls  *json.RawMessage `db:"tool_calls"`
		ToolCallID string           `db:"tool_call_id"`
		AgentName  string           `db:"agent_name"`
	}

	messagesQuery := `
		SELECT role, content, tool_calls, tool_call_id, agent_name
		FROM llm_messages
		WHERE thread_id = $1
		ORDER BY sequence_num ASC
	`

	err = p.db.SelectContext(ctx, &dbMessages, messagesQuery, threadID)
	if err != nil && err != sql.ErrNoRows {
		p.log.Error(ctx, "failed to get messages for agent thread", "threadID", threadID, "ERROR", err)
		return nil, false
	}

	// Create in-memory thread from database data
	agentThread := core.NewInMemoryThreadWithID(threadID)

	// Build messages slice - InMemoryThread already has Messages field we can access
	messages := make([]core.Message, 0, len(dbMessages))
	for _, msg := range dbMessages {
		// Unmarshal tool calls
		var toolCalls []core.ToolCall
		if msg.ToolCalls != nil {
			if err := json.Unmarshal(*msg.ToolCalls, &toolCalls); err != nil {
				p.log.Error(ctx, "failed to unmarshal tool calls", "ERROR", err)
			}
		}

		messages = append(messages, core.Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCalls:  toolCalls,
			ToolCallID: msg.ToolCallID,
			AgentName:  msg.AgentName,
		})
	}

	// Add messages to thread using the AddMessages method
	if len(messages) > 0 {
		if err := agentThread.AddMessages(messages); err != nil {
			p.log.Error(ctx, "failed to add messages to thread", "threadID", threadID, "ERROR", err)
			return nil, false
		}
	}

	return agentThread, true
}

// GetOrCreateAgentThread ensures a thread exists, creating an empty one if needed.
func (p *PostgresLLMContextProjection) GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error) {
	if agentThread, exists := p.GetAgentThread(threadID); exists {
		return agentThread, nil
	}

	return core.NewInMemoryThreadWithID(threadID), nil
}

// Messages retrieves all messages from a thread.
func (p *PostgresLLMContextProjection) Messages(threadID string) []domain.Message {
	if thread, exists := p.Get(threadID); exists {
		return thread.Messages
	}
	return []domain.Message{}
}

// All retrieves all non-deleted threads.
func (p *PostgresLLMContextProjection) All() []domain.Thread {
	ctx := context.Background()

	var threadIDs []string
	query := `
		SELECT thread_id
		FROM llm_threads
		WHERE is_deleted = false
		ORDER BY updated_at DESC
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

// Update is kept for interface compatibility.
func (p *PostgresLLMContextProjection) Update(event core.Event) error {
	ctx := context.Background()
	return p.Apply(ctx, event)
}
