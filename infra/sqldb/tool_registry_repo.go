package sqldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"school-with-ai/services/toolregistry"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// PostgresToolRegistryRepository implements the Tool Registry Repository using PostgreSQL.
type PostgresToolRegistryRepository struct {
	db *sqlx.DB
}

// NewPostgresToolRegistryRepository creates a new PostgresToolRegistryRepository.
func NewPostgresToolRegistryRepository(db *sqlx.DB) *PostgresToolRegistryRepository {
	return &PostgresToolRegistryRepository{
		db: db,
	}
}

// =============================================================================
// Tool Methods
// =============================================================================

// GetTool retrieves a tool by ID.
func (r *PostgresToolRegistryRepository) GetTool(ctx context.Context, id string) (*toolregistry.Tool, error) {
	const q = `SELECT * FROM tools WHERE id = $1`

	var dest struct {
		ID          string          `db:"id"`
		Name        string          `db:"name"`
		Description string          `db:"description"`
		Category    string          `db:"category"`
		ToolSetID   sql.NullString  `db:"tool_set_id"`
		Metadata    json.RawMessage `db:"metadata"`
		CreatedAt   sql.NullTime    `db:"created_at"`
		UpdatedAt   sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.GetContext(ctx, &dest, q, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool with id %s not found", id)
		}
		return nil, fmt.Errorf("selecting tool by id: %w", err)
	}

	tool := &toolregistry.Tool{
		ID:          dest.ID,
		Name:        dest.Name,
		Description: dest.Description,
		Category:    dest.Category,
		CreatedAt:   dest.CreatedAt.Time,
		UpdatedAt:   dest.UpdatedAt.Time,
	}

	if dest.ToolSetID.Valid {
		tool.ToolSetID = dest.ToolSetID.String
	}

	if len(dest.Metadata) > 0 {
		if err := json.Unmarshal(dest.Metadata, &tool.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshalling metadata: %w", err)
		}
	}

	return tool, nil
}

// ListTools returns all tools.
func (r *PostgresToolRegistryRepository) ListTools(ctx context.Context) ([]toolregistry.Tool, error) {
	const q = `SELECT * FROM tools ORDER BY name`

	var dests []struct {
		ID          string          `db:"id"`
		Name        string          `db:"name"`
		Description string          `db:"description"`
		Category    string          `db:"category"`
		ToolSetID   sql.NullString  `db:"tool_set_id"`
		Metadata    json.RawMessage `db:"metadata"`
		CreatedAt   sql.NullTime    `db:"created_at"`
		UpdatedAt   sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.SelectContext(ctx, &dests, q); err != nil {
		return nil, fmt.Errorf("selecting tools: %w", err)
	}

	tools := make([]toolregistry.Tool, len(dests))
	for i, dest := range dests {
		tools[i] = toolregistry.Tool{
			ID:          dest.ID,
			Name:        dest.Name,
			Description: dest.Description,
			Category:    dest.Category,
			CreatedAt:   dest.CreatedAt.Time,
			UpdatedAt:   dest.UpdatedAt.Time,
		}

		if dest.ToolSetID.Valid {
			tools[i].ToolSetID = dest.ToolSetID.String
		}

		if len(dest.Metadata) > 0 {
			if err := json.Unmarshal(dest.Metadata, &tools[i].Metadata); err != nil {
				return nil, fmt.Errorf("unmarshalling metadata for tool %s: %w", dest.ID, err)
			}
		}
	}

	return tools, nil
}

// SaveTool saves or updates a tool.
func (r *PostgresToolRegistryRepository) SaveTool(ctx context.Context, tool *toolregistry.Tool) error {
	const q = `
		INSERT INTO tools (
			id, name, description, category, tool_set_id, metadata, created_at, updated_at
		) VALUES (
			:id, :name, :description, :category, :tool_set_id, :metadata, :created_at, :updated_at
		) ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			tool_set_id = EXCLUDED.tool_set_id,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`

	metadataJSON, err := json.Marshal(tool.Metadata)
	if err != nil {
		return fmt.Errorf("marshalling metadata: %w", err)
	}

	// Handle empty/nil map marshalling to null or empty json object
	if len(tool.Metadata) == 0 {
		metadataJSON = []byte("{}")
	}

	var toolSetID sql.NullString
	if tool.ToolSetID != "" {
		toolSetID.String = tool.ToolSetID
		toolSetID.Valid = true
	}

	params := map[string]interface{}{
		"id":          tool.ID,
		"name":        tool.Name,
		"description": tool.Description,
		"category":    tool.Category,
		"tool_set_id": toolSetID,
		"metadata":    metadataJSON,
		"created_at":  tool.CreatedAt,
		"updated_at":  tool.UpdatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, q, params); err != nil {
		return fmt.Errorf("saving tool: %w", err)
	}

	return nil
}

// DeleteTool removes a tool by ID.
func (r *PostgresToolRegistryRepository) DeleteTool(ctx context.Context, id string) error {
	const q = `DELETE FROM tools WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("deleting tool: %w", err)
	}

	return nil
}

// =============================================================================
// Tool Set Methods
// =============================================================================

// GetToolSet retrieves a tool set by ID.
func (r *PostgresToolRegistryRepository) GetToolSet(ctx context.Context, id string) (*toolregistry.ToolSet, error) {
	const q = `SELECT * FROM tool_sets WHERE id = $1`

	var dest struct {
		ID          string          `db:"id"`
		Name        string          `db:"name"`
		Description string          `db:"description"`
		Category    string          `db:"category"`
		ToolIDs     pq.StringArray  `db:"tool_ids"`
		Metadata    json.RawMessage `db:"metadata"`
		CreatedAt   sql.NullTime    `db:"created_at"`
		UpdatedAt   sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.GetContext(ctx, &dest, q, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool set with id %s not found", id)
		}
		return nil, fmt.Errorf("selecting tool set by id: %w", err)
	}

	toolSet := &toolregistry.ToolSet{
		ID:          dest.ID,
		Name:        dest.Name,
		Description: dest.Description,
		Category:    dest.Category,
		ToolIDs:     []string(dest.ToolIDs),
		CreatedAt:   dest.CreatedAt.Time,
		UpdatedAt:   dest.UpdatedAt.Time,
	}

	if len(dest.Metadata) > 0 {
		if err := json.Unmarshal(dest.Metadata, &toolSet.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshalling metadata: %w", err)
		}
	}

	return toolSet, nil
}

// ListToolSets returns all tool sets.
func (r *PostgresToolRegistryRepository) ListToolSets(ctx context.Context) ([]toolregistry.ToolSet, error) {
	const q = `SELECT * FROM tool_sets ORDER BY name`

	var dests []struct {
		ID          string          `db:"id"`
		Name        string          `db:"name"`
		Description string          `db:"description"`
		Category    string          `db:"category"`
		ToolIDs     pq.StringArray  `db:"tool_ids"`
		Metadata    json.RawMessage `db:"metadata"`
		CreatedAt   sql.NullTime    `db:"created_at"`
		UpdatedAt   sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.SelectContext(ctx, &dests, q); err != nil {
		return nil, fmt.Errorf("selecting tool sets: %w", err)
	}

	toolSets := make([]toolregistry.ToolSet, len(dests))
	for i, dest := range dests {
		toolSets[i] = toolregistry.ToolSet{
			ID:          dest.ID,
			Name:        dest.Name,
			Description: dest.Description,
			Category:    dest.Category,
			ToolIDs:     []string(dest.ToolIDs),
			CreatedAt:   dest.CreatedAt.Time,
			UpdatedAt:   dest.UpdatedAt.Time,
		}

		if len(dest.Metadata) > 0 {
			if err := json.Unmarshal(dest.Metadata, &toolSets[i].Metadata); err != nil {
				return nil, fmt.Errorf("unmarshalling metadata for tool set %s: %w", dest.ID, err)
			}
		}
	}

	return toolSets, nil
}

// SaveToolSet saves or updates a tool set.
func (r *PostgresToolRegistryRepository) SaveToolSet(ctx context.Context, toolSet *toolregistry.ToolSet) error {
	const q = `
		INSERT INTO tool_sets (
			id, name, description, category, tool_ids, metadata, created_at, updated_at
		) VALUES (
			:id, :name, :description, :category, :tool_ids, :metadata, :created_at, :updated_at
		) ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			tool_ids = EXCLUDED.tool_ids,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`

	metadataJSON, err := json.Marshal(toolSet.Metadata)
	if err != nil {
		return fmt.Errorf("marshalling metadata: %w", err)
	}

	if len(toolSet.Metadata) == 0 {
		metadataJSON = []byte("{}")
	}

	// Prepare data for proper types including pq.StringArray

	// We need to pass a struct or map that has the correct types
	// Using map with pq.Array won't work easily with NamedExec and :param syntax directly if we want to be safe with sqlx
	// But sqlx should handle slice to array conversion if the driver supports it, however pq.StringArray is safer.

	params := map[string]interface{}{
		"id":          toolSet.ID,
		"name":        toolSet.Name,
		"description": toolSet.Description,
		"category":    toolSet.Category,
		"tool_ids":    pq.StringArray(toolSet.ToolIDs),
		"metadata":    metadataJSON,
		"created_at":  toolSet.CreatedAt,
		"updated_at":  toolSet.UpdatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, q, params); err != nil {
		// Check for specific errors if needed
		if strings.Contains(err.Error(), "pq: invalid input syntax for type json") {
			// Try to handle potentially nil metadata
		}
		return fmt.Errorf("saving tool set: %w", err)
	}

	return nil
}

// DeleteToolSet removes a tool set by ID.
func (r *PostgresToolRegistryRepository) DeleteToolSet(ctx context.Context, id string) error {
	const q = `DELETE FROM tool_sets WHERE id = $1`

	if _, err := r.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("deleting tool set: %w", err)
	}

	return nil
}
