package agui

import (
	"aicompass/components/i18n"
	"aicompass/foundation/agent"
	"aicompass/foundation/event"
	"aicompass/foundation/logger"
	"aicompass/modules/chat/thread"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
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

// Helper functions for extracting values from metadata maps
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case int64:
			return int(v)
		}
	}
	return 0
}

// RegisterHandlers registers database update handlers to the provided router.
func (p *PostgresAguiProjection) RegisterHandlers(router interface {
	Register(eventPrototype any, handler event.HandlerFunc)
},
) {
	router.Register(&thread.ThreadCreated{}, func(ctx context.Context, e thread.Event) error {
		return p.applyThreadCreated(ctx, e.(*thread.ThreadCreated))
	})
	router.Register(&thread.UserMessageAdded{}, func(ctx context.Context, e thread.Event) error {
		return p.applyUserMessageAdded(ctx, e.(*thread.UserMessageAdded))
	})
	router.Register(&thread.AssistantMessageAdded{}, func(ctx context.Context, e thread.Event) error {
		return p.applyAssistantMessageAdded(ctx, e.(*thread.AssistantMessageAdded))
	})
	router.Register(&thread.ThreadTitleUpdated{}, func(ctx context.Context, e thread.Event) error {
		return p.applyThreadTitleChanged(ctx, e.(*thread.ThreadTitleUpdated))
	})
	router.Register(&thread.ThreadDeleted{}, func(ctx context.Context, e thread.Event) error {
		return p.applyThreadDeleted(ctx, e.(*thread.ThreadDeleted))
	})
	router.Register(&thread.TokenUsageReported{}, func(ctx context.Context, e thread.Event) error {
		return p.applyTokenUsageReported(ctx, e.(*thread.TokenUsageReported))
	})
	// Tool lifecycle events
	router.Register(&thread.ToolStart{}, func(ctx context.Context, e thread.Event) error {
		return p.applyToolStart(ctx, e.(*thread.ToolStart))
	})
	router.Register(&thread.ToolResultAdded{}, func(ctx context.Context, e thread.Event) error {
		return p.applyToolResultAdded(ctx, e.(*thread.ToolResultAdded))
	})
	router.Register(&thread.ToolNeedsInput{}, func(ctx context.Context, e thread.Event) error {
		return p.applyToolNeedsInput(ctx, e.(*thread.ToolNeedsInput))
	})
	router.Register(&thread.ToolNeedsAuth{}, func(ctx context.Context, e thread.Event) error {
		return p.applyToolNeedsAuth(ctx, e.(*thread.ToolNeedsAuth))
	})
	router.Register(&thread.InputProvided{}, func(ctx context.Context, e thread.Event) error {
		return p.applyInputProvided(ctx, e.(*thread.InputProvided))
	})
	router.Register(&thread.AuthGranted{}, func(ctx context.Context, e thread.Event) error {
		return p.applyAuthGranted(ctx, e.(*thread.AuthGranted))
	})
	router.Register(&thread.RunStarted{}, func(ctx context.Context, e thread.Event) error {
		return p.applyRunStarted(ctx, e.(*thread.RunStarted))
	})
	router.Register(&thread.RunFinished{}, func(ctx context.Context, e thread.Event) error {
		return p.applyRunFinished(ctx, e.(*thread.RunFinished))
	})
}

// applyThreadCreated creates a new thread record.
func (p *PostgresAguiProjection) applyThreadCreated(ctx context.Context, event *thread.ThreadCreated) error {
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

// applyUserMessageAdded adds a user message to the thread.
func (p *PostgresAguiProjection) applyUserMessageAdded(ctx context.Context, event *thread.UserMessageAdded) error {
	// Marshal attachments to metadata JSONB for persistence
	var metadataJSON []byte
	if len(event.Attachments) > 0 {
		metaMap := map[string]any{"attachments": event.Attachments}
		var err error
		metadataJSON, err = json.Marshal(metaMap)
		if err != nil {
			p.log.Warn(ctx, "failed to marshal attachment metadata", "error", err)
		}
	}

	// Use atomic subquery for sequence number to prevent race conditions
	// under concurrent event processing.
	insertQuery := `
		INSERT INTO agui_messages (message_id, thread_id, role, content, metadata, created_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, (SELECT COUNT(*) FROM agui_messages WHERE thread_id = $2::varchar))
		ON CONFLICT (message_id) DO NOTHING
	`

	_, err := p.db.ExecContext(ctx, insertQuery,
		event.MessageID,
		event.ThreadID,
		"user",
		event.Content,
		metadataJSON,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert user message: %w", err)
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

// applyAssistantMessageAdded adds an assistant message to the thread.
func (p *PostgresAguiProjection) applyAssistantMessageAdded(ctx context.Context, event *thread.AssistantMessageAdded) error {
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
		INSERT INTO agui_messages (message_id, thread_id, role, content, tool_calls, agent_name, created_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT COUNT(*) FROM agui_messages WHERE thread_id = $2::varchar))
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
		return fmt.Errorf("failed to insert assistant message: %w", err)
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

// applyToolResultAdded adds a tool result message to the thread.
func (p *PostgresAguiProjection) applyToolResultAdded(ctx context.Context, event *thread.ToolResultAdded) error {
	// Use atomic subquery for sequence number to prevent race conditions.
	insertQuery := `
		INSERT INTO agui_messages (message_id, thread_id, role, content, tool_call_id, agent_name, created_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6, $7, (SELECT COUNT(*) FROM agui_messages WHERE thread_id = $2::varchar))
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
		return fmt.Errorf("failed to insert tool result message: %w", err)
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

	// Update tool state to terminal state in dedicated table.
	// Guard: do not overwrite elicitation lifecycle states (awaiting_input,
	// input_provided, answered) — those are more meaningful than the
	// pause-handler's "completed" placeholder for elicitation tools.
	terminalState := "completed"
	if event.IsError {
		terminalState = "error"
	}
	toolStateQuery := `
		INSERT INTO agui_tool_states (tool_call_id, thread_id, state, tool_name, agent_name, result, error, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $7 THEN $6 ELSE NULL END, $8)
		ON CONFLICT (tool_call_id) DO UPDATE SET
			state = CASE
				WHEN agui_tool_states.state IN ('awaiting_input', 'input_provided', 'answered')
				THEN agui_tool_states.state
				ELSE EXCLUDED.state
			END,
			result = CASE
				WHEN agui_tool_states.state IN ('awaiting_input', 'input_provided', 'answered')
				THEN agui_tool_states.result
				ELSE EXCLUDED.result
			END,
			error = CASE
				WHEN agui_tool_states.state IN ('awaiting_input', 'input_provided', 'answered')
				THEN agui_tool_states.error
				ELSE EXCLUDED.error
			END,
			updated_at = EXCLUDED.updated_at
	`
	if _, err := p.db.ExecContext(ctx, toolStateQuery,
		event.ToolCallID,
		event.ThreadID,
		terminalState,
		event.ToolName,
		event.AgentName,
		event.Result,
		event.IsError,
		event.Timestamp,
	); err != nil {
		p.log.Error(ctx, "failed to upsert tool result state", "toolCallId", event.ToolCallID, "ERROR", err)
		// Non-fatal: the message row was inserted successfully
	}

	return nil
}

// applyThreadTitleChanged updates the thread title.
func (p *PostgresAguiProjection) applyThreadTitleChanged(ctx context.Context, event *thread.ThreadTitleUpdated) error {
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
func (p *PostgresAguiProjection) applyThreadDeleted(ctx context.Context, event *thread.ThreadDeleted) error {
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

// applyTokenUsageReported updates message metadata with usage information.
func (p *PostgresAguiProjection) applyTokenUsageReported(ctx context.Context, event *thread.TokenUsageReported) error {
	usageJSON, err := json.Marshal(map[string]any{
		"usage": map[string]any{
			"provider":        event.Provider,
			"model":           event.Model,
			"prompt":          event.PromptTokens,
			"completion":      event.CompletionTokens,
			"total":           event.TotalTokens,
			"cached":          event.CachedTokens,
			"reasoning":       event.ReasoningTokens,
			"audioInput":      event.AudioInputTokens,
			"audioOutput":     event.AudioOutputTokens,
			"acceptedPredict": event.AcceptedPredictionTokens,
			"rejectedPredict": event.RejectedPredictionTokens,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal usage: %w", err)
	}

	query := `
		UPDATE agui_messages
		SET metadata = CASE 
			WHEN metadata IS NULL THEN $1 
			ELSE metadata || $1 
		END
		WHERE message_id = $2
	`

	_, err = p.db.ExecContext(ctx, query, usageJSON, event.MessageID)
	if err != nil {
		return fmt.Errorf("failed to update message usage metadata: %w", err)
	}

	return nil
}

// applyToolStart records the tool execution start in the dedicated tool states table.
func (p *PostgresAguiProjection) applyToolStart(ctx context.Context, event *thread.ToolStart) error {
	query := `
		INSERT INTO agui_tool_states (tool_call_id, thread_id, state, tool_name, agent_name, arguments, updated_at)
		VALUES ($1, $2, 'executing', $3, $4, $5, $6)
		ON CONFLICT (tool_call_id) DO UPDATE SET
			state = 'executing',
			tool_name  = COALESCE(EXCLUDED.tool_name, agui_tool_states.tool_name),
			agent_name = COALESCE(EXCLUDED.agent_name, agui_tool_states.agent_name),
			arguments  = COALESCE(EXCLUDED.arguments, agui_tool_states.arguments),
			updated_at = EXCLUDED.updated_at
	`
	_, err := p.db.ExecContext(ctx, query,
		event.ToolCallID,
		event.ThreadID,
		event.ToolName,
		event.AgentName,
		event.ToolArguments,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert tool start: %w", err)
	}
	return nil
}

// applyToolNeedsInput records elicitation data in the dedicated tool states table.
func (p *PostgresAguiProjection) applyToolNeedsInput(ctx context.Context, event *thread.ToolNeedsInput) error {
	elicitationJSON, err := json.Marshal(event.Elicitation)
	if err != nil {
		return fmt.Errorf("failed to marshal elicitation: %w", err)
	}

	query := `
		INSERT INTO agui_tool_states (tool_call_id, thread_id, state, tool_name, agent_name, elicitation, updated_at)
		VALUES ($1, $2, 'awaiting_input', $3, $4, $5, $6)
		ON CONFLICT (tool_call_id) DO UPDATE SET
			state       = 'awaiting_input',
			elicitation = EXCLUDED.elicitation,
			updated_at  = EXCLUDED.updated_at
	`
	_, err = p.db.ExecContext(ctx, query,
		event.ToolCallID,
		event.ThreadID,
		event.ToolName,
		event.AgentName,
		elicitationJSON,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert tool awaiting input: %w", err)
	}
	return nil
}

// applyToolNeedsAuth records authorization data in the dedicated tool states table.
func (p *PostgresAguiProjection) applyToolNeedsAuth(ctx context.Context, event *thread.ToolNeedsAuth) error {
	authSpecJSON, err := json.Marshal(event.AuthSpec)
	if err != nil {
		return fmt.Errorf("failed to marshal auth spec: %w", err)
	}

	query := `
		INSERT INTO agui_tool_states (tool_call_id, thread_id, state, tool_name, agent_name, auth_spec, updated_at)
		VALUES ($1, $2, 'awaiting_auth', $3, $4, $5, $6)
		ON CONFLICT (tool_call_id) DO UPDATE SET
			state      = 'awaiting_auth',
			auth_spec  = EXCLUDED.auth_spec,
			updated_at = EXCLUDED.updated_at
	`
	_, err = p.db.ExecContext(ctx, query,
		event.ToolCallID,
		event.ThreadID,
		event.ToolName,
		event.AgentName,
		authSpecJSON,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert tool awaiting auth: %w", err)
	}
	return nil
}

// applyInputProvided records the user's answer in the dedicated tool states table.
// With the dedicated table this is a simple UPSERT by tool_call_id — no JSONB
// containment query, no ordering dependency on UserMessageAdded.
func (p *PostgresAguiProjection) applyInputProvided(ctx context.Context, event *thread.InputProvided) error {
	query := `
		INSERT INTO agui_tool_states (tool_call_id, thread_id, state, input, agent_name, updated_at)
		VALUES ($1, $2, 'input_provided', $3, $4, $5)
		ON CONFLICT (tool_call_id) DO UPDATE SET
			state      = 'input_provided',
			input      = EXCLUDED.input,
			updated_at = EXCLUDED.updated_at
	`
	_, err := p.db.ExecContext(ctx, query,
		event.ToolCallID,
		event.ThreadID,
		event.Input,
		event.AgentName,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert tool input provided: %w", err)
	}
	return nil
}

// applyAuthGranted records authorization grant in the dedicated tool states table.
func (p *PostgresAguiProjection) applyAuthGranted(ctx context.Context, event *thread.AuthGranted) error {
	query := `
		INSERT INTO agui_tool_states (tool_call_id, thread_id, state, agent_name, updated_at)
		VALUES ($1, $2, 'auth_granted', $3, $4)
		ON CONFLICT (tool_call_id) DO UPDATE SET
			state      = 'auth_granted',
			updated_at = EXCLUDED.updated_at
	`
	_, err := p.db.ExecContext(ctx, query,
		event.ToolCallID,
		event.ThreadID,
		event.AgentName,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert tool auth granted: %w", err)
	}
	return nil
}

// GetThreadState retrieves the complete UI state for a thread from PostgreSQL.
func (p *PostgresAguiProjection) GetThreadState(ctx context.Context, threadID string) (*AguiThreadState, bool) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get thread record
	var threadRow struct {
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

	err := p.db.GetContext(ctx, &threadRow, threadQuery, threadID)
	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		p.log.Error(ctx, "failed to get thread", "threadID", threadID, "ERROR", err)
		return nil, false
	}

	// Don't return deleted threads
	if threadRow.IsDeleted {
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
	messageAttachments := make(map[string][]thread.Attachment)
	var tokenUsage *TokenUsageInfo

	for _, msg := range messages {
		content := msg.Content
		aguimsit := aguievents.Message{
			ID:      msg.MessageID,
			Role:    types.Role(msg.Role),
			Content: &content,
		}

		if msg.ToolCallID != nil {
			aguimsit.ToolCallID = *msg.ToolCallID
		}

		if msg.AgentName != nil {
			aguimsit.Name = *msg.AgentName
		}

		if msg.ToolCalls != nil {
			var agentToolCalls []agent.ToolCall
			if err := json.Unmarshal([]byte(*msg.ToolCalls), &agentToolCalls); err == nil {
				aguiToolCalls := make([]aguievents.ToolCall, 0, len(agentToolCalls))
				for _, tc := range agentToolCalls {
					aguiToolCalls = append(aguiToolCalls, aguievents.ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      tc.Name,
							Arguments: tc.Arguments,
						},
					})
				}
				aguimsit.ToolCalls = aguiToolCalls
			}
		}

		// Extract metadata (token usage, images, etc.)
		if msg.Metadata != nil {
			var metadata map[string]interface{}
			if err := json.Unmarshal([]byte(*msg.Metadata), &metadata); err == nil {
				// Extract token usage
				if usage, ok := metadata["usage"].(map[string]interface{}); ok {
					tokenUsage = &TokenUsageInfo{
						Provider:        getStringFromMap(usage, "provider"),
						Model:           getStringFromMap(usage, "model"),
						Prompt:          getIntFromMap(usage, "prompt"),
						Completion:      getIntFromMap(usage, "completion"),
						Total:           getIntFromMap(usage, "total"),
						Cached:          getIntFromMap(usage, "cached"),
						Reasoning:       getIntFromMap(usage, "reasoning"),
						AudioInput:      getIntFromMap(usage, "audioInput"),
						AudioOutput:     getIntFromMap(usage, "audioOutput"),
						AcceptedPredict: getIntFromMap(usage, "acceptedPredict"),
						RejectedPredict: getIntFromMap(usage, "rejectedPredict"),
						MessageID:       msg.MessageID,
						ThreadID:        threadID,
					}
				}

				// Extract attachment references for user messages
				if attachmentsRaw, ok := metadata["attachments"]; ok {
					if attachmentsArr, ok := attachmentsRaw.([]interface{}); ok {
						var refs []thread.Attachment
						for _, attRaw := range attachmentsArr {
							if attMap, ok := attRaw.(map[string]interface{}); ok {
								ref := thread.Attachment{
									URL:      getStringFromMap(attMap, "url"),
									MimeType: getStringFromMap(attMap, "mimeType"),
									FileName: getStringFromMap(attMap, "fileName"),
								}
								if ref.URL != "" {
									refs = append(refs, ref)
								}
							}
						}
						if len(refs) > 0 {
							messageAttachments[msg.MessageID] = refs
						}
					}
				}
			}
		}

		aguiMessages = append(aguiMessages, aguimsit)
	}

	// Fetch tool states from the dedicated table (no merge logic needed)
	var toolStateRows []struct {
		ToolCallID  string           `db:"tool_call_id"`
		State       string           `db:"state"`
		ToolName    *string          `db:"tool_name"`
		AgentName   *string          `db:"agent_name"`
		Elicitation *json.RawMessage `db:"elicitation"`
		Input       *string          `db:"input"`
		Result      *string          `db:"result"`
		Error       *string          `db:"error"`
	}

	toolStatesQuery := `
		SELECT tool_call_id, state, tool_name, agent_name, elicitation, input, result, error
		FROM agui_tool_states
		WHERE thread_id = $1
	`
	if err := p.db.SelectContext(ctx, &toolStateRows, toolStatesQuery, threadID); err != nil && err != sql.ErrNoRows {
		p.log.Error(ctx, "failed to get tool states", "threadID", threadID, "ERROR", err)
	}

	toolStatesMap := make(map[string]interface{}, len(toolStateRows))
	for _, ts := range toolStateRows {
		entry := map[string]interface{}{
			"toolCallId": ts.ToolCallID,
			"state":      ts.State,
		}
		if ts.ToolName != nil {
			entry["toolName"] = *ts.ToolName
		}
		if ts.AgentName != nil {
			entry["agentName"] = *ts.AgentName
		}
		if ts.Elicitation != nil {
			var elicitation interface{}
			if err := json.Unmarshal(*ts.Elicitation, &elicitation); err == nil {
				entry["elicitation"] = elicitation
			}
		}
		if ts.Input != nil {
			entry["input"] = *ts.Input
		}
		if ts.Result != nil {
			entry["result"] = *ts.Result
		}
		if ts.Error != nil {
			entry["error"] = *ts.Error
		}
		toolStatesMap[ts.ToolCallID] = entry
	}

	state := &AguiThreadState{
		ThreadID:           threadRow.ThreadID,
		UserID:             threadRow.UserID,
		Title:              threadRow.Title,
		Messages:           aguiMessages,
		ToolCalls:          []AguiToolCall{},
		AgentActivity:      make(map[string]*AguiAgentActivity),
		ToolStates:         toolStatesMap,
		TokenUsage:         tokenUsage,
		MessageAttachments: messageAttachments,
		CreatedAt:          threadRow.CreatedAt,
		UpdatedAt:          threadRow.UpdatedAt,
		Version:            threadRow.Version,
		IsDeleted:          threadRow.IsDeleted,
	}

	return state, true
}

// GetMessages retrieves all UI-formatted messages for a thread.
func (p *PostgresAguiProjection) GetMessages(ctx context.Context, threadID string) []aguievents.Message {
	state, exists := p.GetThreadState(ctx, threadID)
	if !exists {
		return []aguievents.Message{}
	}
	return state.Messages
}

// ListThreads returns paginated threads for a user.
func (p *PostgresAguiProjection) ListThreads(ctx context.Context, req ThreadListRequest) (*ThreadListResponse, error) {
	// Apply defaults and limits
	pageSize := req.Limit
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// Fetch one extra to determine if there are more pages
	fetchLimit := pageSize + 1

	var threads []struct {
		ThreadID  string    `db:"thread_id"`
		Title     *string   `db:"title"`
		UpdatedAt time.Time `db:"updated_at"`
	}

	var err error
	if req.Query != "" {
		// Search mode - filter by title.
		// Escape LIKE wildcards to prevent pattern injection (e.g. % or _).
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(req.Query)
		searchPattern := "%" + escaped + "%"
		if req.Cursor == "" {
			query := `
				SELECT thread_id, title, updated_at
				FROM agui_threads
				WHERE user_id = $1 AND is_deleted = false AND title ILIKE $2
				ORDER BY updated_at DESC
				LIMIT $3
			`
			err = p.db.SelectContext(ctx, &threads, query, req.UserID, searchPattern, fetchLimit)
		} else {
			cursorTime, parseErr := time.Parse(time.RFC3339Nano, req.Cursor)
			if parseErr != nil {
				return nil, fmt.Errorf("invalid cursor format: %w", parseErr)
			}
			query := `
				SELECT thread_id, title, updated_at
				FROM agui_threads
				WHERE user_id = $1 AND is_deleted = false AND title ILIKE $2 AND updated_at < $3
				ORDER BY updated_at DESC
				LIMIT $4
			`
			err = p.db.SelectContext(ctx, &threads, query, req.UserID, searchPattern, cursorTime, fetchLimit)
		}
	} else if req.Cursor == "" {
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
	hasMore := len(threads) > pageSize
	if hasMore {
		threads = threads[:pageSize] // Trim the extra item
	}

	// Convert to response format
	result := &ThreadListResponse{
		Threads: make([]ThreadSummary, 0, len(threads)),
		HasMore: hasMore,
	}

	now := time.Now()
	today := now.Truncate(24 * time.Hour)
	yesterday := today.Add(-24 * time.Hour)
	sevenDaysAgo := today.Add(-7 * 24 * time.Hour)
	thirtyDaysAgo := today.Add(-30 * 24 * time.Hour)

	for _, t := range threads {
		title := ""
		if t.Title != nil {
			title = *t.Title
		}

		// Calculate group title
		var groupTitle string
		switch {
		case t.UpdatedAt.After(today):
			groupTitle = i18n.T(ctx, "Today")
		case t.UpdatedAt.After(yesterday):
			groupTitle = i18n.T(ctx, "Yesterday")
		case t.UpdatedAt.After(sevenDaysAgo):
			groupTitle = i18n.T(ctx, "Previous 7 Days")
		case t.UpdatedAt.After(thirtyDaysAgo):
			groupTitle = i18n.T(ctx, "Previous 30 Days")
		default:
			groupTitle = i18n.T(ctx, "Older")
		}

		result.Threads = append(result.Threads, ThreadSummary{
			ID:         t.ThreadID,
			Title:      title,
			GroupTitle: groupTitle,
			UpdatedAt:  t.UpdatedAt,
		})
	}

	// Set next cursor if there are more pages
	if hasMore && len(result.Threads) > 0 {
		lastThread := result.Threads[len(result.Threads)-1]
		result.NextCursor = lastThread.UpdatedAt.Format(time.RFC3339Nano)
	}

	return result, nil
}

// applyRunStarted handles run start events by updating the thread metadata.
func (p *PostgresAguiProjection) applyRunStarted(ctx context.Context, event *thread.RunStarted) error {
	// For now, we don't need to store run start/end separately in the database
	// The frontend just needs these events for UI state management
	// We could add run tracking to the metadata if needed later
	return nil
}

// applyRunFinished handles run completion events by updating the thread metadata.
func (p *PostgresAguiProjection) applyRunFinished(ctx context.Context, event *thread.RunFinished) error {
	// For now, we don't need to store run start/end separately in the database
	// The frontend just needs these events for UI state management
	// We could add run tracking to the metadata if needed later
	return nil
}

// Verify PostgresAguiProjection implements AguiProjection.
var _ AguiProjection = (*PostgresAguiProjection)(nil)
