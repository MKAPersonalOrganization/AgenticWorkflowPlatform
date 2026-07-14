// Package memdb provides a persistent PostgreSQL-backed implementation of the
// ADK-Go memory.Service interface.  It stores session events in a dedicated
// table and uses PostgreSQL full-text search (tsvector / plainto_tsquery) for
// keyword-based recall.
package memdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// Service implements memory.Service using PostgreSQL with full-text search.
type Service struct {
	db *sqlx.DB
}

// New creates a persistent memory service backed by the given *sqlx.DB.
// The caller must ensure the memory_entries table exists (run migration first).
func New(db *sqlx.DB) *Service {
	return &Service{db: db}
}

// AddSessionToMemory extracts text from every event in the session and
// upserts them into the memory_entries table.  If the same session is
// added again, its previous entries are replaced (idempotent).
func (s *Service) AddSessionToMemory(ctx context.Context, sess session.Session) error {
	appName := sess.AppName()
	userID := sess.UserID()
	sessionID := sess.ID()

	// Delete previous entries for this session so re-indexing is idempotent.
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM memory_entries WHERE app_name = $1 AND user_id = $2 AND session_id = $3`,
		appName, userID, sessionID,
	)
	if err != nil {
		return fmt.Errorf("memory: clear old entries: %w", err)
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("memory: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PreparexContext(ctx, `
		INSERT INTO memory_entries (id, app_name, user_id, session_id, event_id, author, content, content_text, custom_metadata, event_timestamp)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return fmt.Errorf("memory: prepare insert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for event := range sess.Events().All() {
		if event.Content == nil {
			continue
		}

		text := extractText(event.Content)
		if strings.TrimSpace(text) == "" {
			continue
		}

		contentJSON, err := json.Marshal(event.Content)
		if err != nil {
			continue // skip unmarshallable events
		}

		var metaJSON []byte
		if len(event.CustomMetadata) > 0 {
			metaJSON, _ = json.Marshal(event.CustomMetadata)
		}

		_, err = stmt.ExecContext(ctx,
			appName, userID, sessionID,
			event.ID, event.Author,
			contentJSON, text, metaJSON,
			event.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("memory: insert event %s: %w", event.ID, err)
		}
	}

	return tx.Commit()
}

// SearchMemory performs a full-text search across stored memory entries for
// the given user and app.  It uses PostgreSQL's plainto_tsquery to tokenise
// the query and ranks results by relevance (ts_rank).
func (s *Service) SearchMemory(ctx context.Context, req *memory.SearchRequest) (*memory.SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return &memory.SearchResponse{}, nil
	}

	rows, err := s.db.QueryxContext(ctx, `
		SELECT event_id, author, content, custom_metadata, event_timestamp
		FROM memory_entries
		WHERE app_name = $1
		  AND user_id = $2
		  AND to_tsvector('english', content_text) @@ plainto_tsquery('english', $3)
		ORDER BY event_timestamp DESC
		LIMIT 20
	`, req.AppName, req.UserID, query)
	if err != nil {
		return nil, fmt.Errorf("memory: search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []memory.Entry
	for rows.Next() {
		var (
			eventID        string
			author         string
			contentJSON    *json.RawMessage
			customMetaJSON *json.RawMessage
			ts             time.Time
		)
		if err := rows.Scan(&eventID, &author, &contentJSON, &customMetaJSON, &ts); err != nil {
			return nil, fmt.Errorf("memory: scan: %w", err)
		}

		var content *genai.Content
		if contentJSON != nil && len(*contentJSON) > 0 {
			if err := json.Unmarshal(*contentJSON, &content); err != nil {
				continue
			}
		}

		var meta map[string]any
		if customMetaJSON != nil && len(*customMetaJSON) > 0 {
			_ = json.Unmarshal(*customMetaJSON, &meta)
		}

		entries = append(entries, memory.Entry{
			ID:             eventID,
			Content:        content,
			Author:         author,
			Timestamp:      ts,
			CustomMetadata: meta,
		})
	}

	if entries == nil {
		entries = []memory.Entry{}
	}
	return &memory.SearchResponse{Memories: entries}, rows.Err()
}

// extractText concatenates all Text parts from a genai.Content.
func extractText(c *genai.Content) string {
	if c == nil {
		return ""
	}
	var parts []string
	for _, p := range c.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}
