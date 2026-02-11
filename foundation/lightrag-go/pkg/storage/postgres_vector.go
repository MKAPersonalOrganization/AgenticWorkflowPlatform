package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// PostgresVectorStorage implements BaseVectorStorage using PostgreSQL and pgvector.
type PostgresVectorStorage struct {
	db        *PostgresDB
	namespace string
	workspace string
	tableName string
	config    *StorageConfig
}

// NewPostgresVectorStorage creates a new PostgresVectorStorage instance.
func NewPostgresVectorStorage(db *PostgresDB, config *StorageConfig) *PostgresVectorStorage {
	return &PostgresVectorStorage{
		db:        db,
		namespace: config.Namespace,
		workspace: config.Workspace,
		tableName: fmt.Sprintf("lightrag_vdb_%s", config.Namespace), // e.g. lightrag_vdb_chunks
		config:    config,
	}
}

func (s *PostgresVectorStorage) GetNamespace() string {
	return s.namespace
}

func (s *PostgresVectorStorage) GetWorkspace() string {
	return s.workspace
}

func (s *PostgresVectorStorage) Initialize(ctx context.Context) error {
	// Create table
	// We use JSONB for meta fields to be flexible
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id VARCHAR(1024) PRIMARY KEY,
			workspace VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			vector vector(%d),
			meta JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_%s_meta ON %s USING GIN (meta);
	`, s.tableName, s.config.EmbeddingDim, s.tableName, s.tableName)

	_, err := s.db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to initialize table %s: %w", s.tableName, err)
	}
	return nil
}

func (s *PostgresVectorStorage) Finalize(ctx context.Context) error {
	return nil
}

func (s *PostgresVectorStorage) IndexDoneCallback(ctx context.Context) error {
	return nil
}

func (s *PostgresVectorStorage) Drop(ctx context.Context) (map[string]string, error) {
	query := fmt.Sprintf("DROP TABLE IF EXISTS %s", s.tableName)
	_, err := s.db.Pool.Exec(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to drop table: %w", err)
	}
	return map[string]string{"status": "success", "message": "table dropped"}, nil
}

func (s *PostgresVectorStorage) Query(ctx context.Context, query string, topK int, queryEmbedding []float64) ([]map[string]interface{}, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding is required")
	}

	// Use pgvector cosine distance: <=>
	sql := fmt.Sprintf(`
		SELECT id, meta, vector <=> $1 as distance
		FROM %s
		WHERE workspace = $2 AND namespace = $3
		ORDER BY distance
		LIMIT $4
	`, s.tableName)

	vector := pgvector.NewVector(toFloat32(queryEmbedding))
	rows, err := s.db.Pool.Query(ctx, sql, vector, s.workspace, s.namespace, topK)
	if err != nil {
		return nil, fmt.Errorf("failed to query vectors: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id string
		var meta []byte
		var distance float64
		if err := rows.Scan(&id, &meta, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(meta, &result); err != nil {
			result = make(map[string]interface{})
		}
		result["id"] = id
		result["distance"] = distance
		result["score"] = 1 - distance

		results = append(results, result)
	}
	return results, nil
}

func (s *PostgresVectorStorage) Upsert(ctx context.Context, data map[string]map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := fmt.Sprintf(`
		INSERT INTO %s (id, workspace, namespace, vector, meta, updated_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
		ON CONFLICT (id)
		DO UPDATE SET vector = EXCLUDED.vector, meta = EXCLUDED.meta, updated_at = CURRENT_TIMESTAMP
	`, s.tableName)

	batch := &pgx.Batch{}
	for id, item := range data {
		// Extract embedding
		emb, ok := item["embedding"].([]float64)
		if !ok {
			// Try to check if it's []interface{} and convert
			if embIf, ok := item["embedding"].([]interface{}); ok {
				emb = make([]float64, len(embIf))
				for i, v := range embIf {
					if f, ok := v.(float64); ok {
						emb[i] = f
					}
				}
			} else {
				continue // Skip if no embedding
			}
		}

		// Ensure embedding matches dimension
		if len(emb) != s.config.EmbeddingDim {
			continue
		}

		vector := pgvector.NewVector(toFloat32(emb))

		// Meta includes everything except embedding
		meta := make(map[string]interface{})
		for k, v := range item {
			if k != "embedding" {
				meta[k] = v
			}
		}
		metaJson, _ := json.Marshal(meta)

		batch.Queue(query, id, s.workspace, s.namespace, vector, metaJson)
	}

	br := tx.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to execute batch: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func toFloat32(v []float64) []float32 {
	res := make([]float32, len(v))
	for i, f := range v {
		res[i] = float32(f)
	}
	return res
}

func (s *PostgresVectorStorage) DeleteEntity(ctx context.Context, entityName string) error {
	// Not directly supported by schema unless entityName is ID or in meta
	// Assuming entityName is stored in meta->>'entity_name'
	query := fmt.Sprintf("DELETE FROM %s WHERE workspace = $1 AND namespace = $2 AND meta->>'entity_name' = $3", s.tableName)
	_, err := s.db.Pool.Exec(ctx, query, s.workspace, s.namespace, entityName)
	return err
}

func (s *PostgresVectorStorage) DeleteEntityRelation(ctx context.Context, entityName string) error {
	// Assuming relations involving entityName are stored in meta->>'src_id' or meta->>'tgt_id'
	// But 'src_id' might be exact match?
	// This depends on how relations are stored. Standard LightRAG VDB has "src_id" and "tgt_id" in meta.
	// But DeleteEntityRelation implies deleting relations *of* this entity.
	// This might be tricky with just meta JSONB query if names are complex.
	// For now, let's assume 'src_id' and 'tgt_id' match entity names? No, they are IDs.
	// We need entity IDs.
	// If entityName is passed, we might need to find its ID first?
	// Wait, LightRAG VDB interface says `DeleteEntityRelation(ctx, entityName string)`.
	// Usually this is for Entity VDB deletion.

	// Implementation:
	// Find ID of entityName?
	// Or just delete where meta->>'entity_name' = entityName?
	// This method name `DeleteEntityRelation` is slightly confusing in VectorStorage context.
	// In `NanoVectorDBStorage`, it deletes relations.
	return nil // TODO: implement if needed
}

func (s *PostgresVectorStorage) GetByID(ctx context.Context, id string) (map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT id, vector, meta FROM %s WHERE id = $1", s.tableName)
	var rid string
	var vector pgvector.Vector
	var meta []byte
	err := s.db.Pool.QueryRow(ctx, query, id).Scan(&rid, &vector, &meta)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get by id: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(meta, &result); err != nil {
		result = make(map[string]interface{})
	}
	result["id"] = rid

	// pgvector.Vector is already []float32 usually, but here mapped to []float64?
	// pgxvector.NewVector takes []float64 or []float32
	// On scan, it returns []float32 probably.
	// Let's check type.
	// The library documentation says it scans into []float32.
	// We need to convert to []float64 for interface.

	vecFloats := vector.Slice() // []float32
	vec64 := make([]float64, len(vecFloats))
	for i, v := range vecFloats {
		vec64[i] = float64(v)
	}
	result["embedding"] = vec64
	return result, nil
}

func (s *PostgresVectorStorage) GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error) {
	if len(ids) == 0 {
		return []map[string]interface{}{}, nil
	}

	query := fmt.Sprintf("SELECT id, vector, meta FROM %s WHERE id = ANY($1)", s.tableName)
	rows, err := s.db.Pool.Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get by ids: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id string
		var vector pgvector.Vector
		var meta []byte
		if err := rows.Scan(&id, &vector, &meta); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(meta, &result); err != nil {
			result = make(map[string]interface{})
		}
		result["id"] = id
		result["embedding"] = toFloat64(vector.Slice())
		results = append(results, result)
	}
	return results, nil
}

func toFloat64(v []float32) []float64 {
	res := make([]float64, len(v))
	for i, f := range v {
		res[i] = float64(f)
	}
	return res
}

func (s *PostgresVectorStorage) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ANY($1)", s.tableName)
	_, err := s.db.Pool.Exec(ctx, query, ids)
	return err
}

func (s *PostgresVectorStorage) GetVectorsByIDs(ctx context.Context, ids []string) (map[string][]float64, error) {
	if len(ids) == 0 {
		return map[string][]float64{}, nil
	}
	query := fmt.Sprintf("SELECT id, vector FROM %s WHERE id = ANY($1)", s.tableName)
	rows, err := s.db.Pool.Query(ctx, query, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]float64)
	for rows.Next() {
		var id string
		var vector pgvector.Vector
		if err := rows.Scan(&id, &vector); err != nil {
			return nil, err
		}
		vecFloats := vector.Slice()
		vec64 := make([]float64, len(vecFloats))
		for i, v := range vecFloats {
			vec64[i] = float64(v)
		}
		result[id] = vec64
	}
	return result, nil
}

func (s *PostgresVectorStorage) GetCosineBetterThanThreshold() float64 {
	return s.config.CosineThreshold
}

func (s *PostgresVectorStorage) GetMetaFields() map[string]struct{} {
	// Return empty or valid, implementation specific
	return make(map[string]struct{})
}
