package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ──────────────────────────────────────────────────────────────────────────────
// PostgreSQL Repository
// ──────────────────────────────────────────────────────────────────────────────

type postgresWorkflowRepository struct {
	db *sqlx.DB
}

func newPostgresWorkflowRepository(db *sqlx.DB) *postgresWorkflowRepository {
	return &postgresWorkflowRepository{db: db}
}

func (r *postgresWorkflowRepository) Create(ctx context.Context, wf *WorkflowDefinition) error {
	if wf.ID == "" {
		wf.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	wf.CreatedAt = now
	wf.UpdatedAt = now

	graphJSON, err := json.Marshal(wf.Graph)
	if err != nil {
		return fmt.Errorf("marshalling graph: %w", err)
	}

	query := `
INSERT INTO workflow_definitions (id, user_id, name, description, icon, graph, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = r.db.ExecContext(ctx, query,
		wf.ID,
		wf.UserID,
		wf.Name,
		wf.Description,
		wf.Icon,
		string(graphJSON),
		wf.CreatedAt,
		wf.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting workflow definition: %w", err)
	}

	return nil
}

func (r *postgresWorkflowRepository) Update(ctx context.Context, wf *WorkflowDefinition) error {
	wf.UpdatedAt = time.Now().UTC()

	graphJSON, err := json.Marshal(wf.Graph)
	if err != nil {
		return fmt.Errorf("marshalling graph: %w", err)
	}

	query := `
UPDATE workflow_definitions
SET name = $1, description = $2, icon = $3, graph = $4, updated_at = $5
WHERE id = $6 AND user_id = $7`

	result, err := r.db.ExecContext(ctx, query,
		wf.Name,
		wf.Description,
		wf.Icon,
		string(graphJSON),
		wf.UpdatedAt,
		wf.ID,
		wf.UserID,
	)
	if err != nil {
		return fmt.Errorf("updating workflow definition: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("workflow not found or not owned by user")
	}

	return nil
}

func (r *postgresWorkflowRepository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM workflow_definitions WHERE id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("deleting workflow definition: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("workflow not found or not owned by user")
	}

	return nil
}

func (r *postgresWorkflowRepository) GetByID(ctx context.Context, id, userID string) (*WorkflowDefinition, error) {
	query := `SELECT id, user_id, name, description, icon, graph, created_at, updated_at
FROM workflow_definitions WHERE id = $1 AND user_id = $2`

	var wf WorkflowDefinition
	if err := r.db.GetContext(ctx, &wf, query, id, userID); err != nil {
		return nil, fmt.Errorf("getting workflow definition: %w", err)
	}

	if err := json.Unmarshal([]byte(wf.GraphJSON), &wf.Graph); err != nil {
		return nil, fmt.Errorf("unmarshalling graph: %w", err)
	}

	return &wf, nil
}

func (r *postgresWorkflowRepository) ListByUser(ctx context.Context, userID string) ([]WorkflowDefinition, error) {
	query := `SELECT id, user_id, name, description, icon, graph, created_at, updated_at
FROM workflow_definitions WHERE user_id = $1 ORDER BY created_at ASC`

	var workflows []WorkflowDefinition
	if err := r.db.SelectContext(ctx, &workflows, query, userID); err != nil {
		return nil, fmt.Errorf("listing workflow definitions: %w", err)
	}

	for i := range workflows {
		if workflows[i].GraphJSON != "" {
			if err := json.Unmarshal([]byte(workflows[i].GraphJSON), &workflows[i].Graph); err != nil {
				return nil, fmt.Errorf("unmarshalling graph for %s: %w", workflows[i].ID, err)
			}
		}
	}

	return workflows, nil
}
