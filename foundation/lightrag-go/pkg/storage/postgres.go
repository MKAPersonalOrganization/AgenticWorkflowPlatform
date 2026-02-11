package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

// PostgresDB handles the database connection pool.
type PostgresDB struct {
	Pool *pgxpool.Pool
}

// NewPostgresDB creates a new PostgresDB instance.
func NewPostgresDB(ctx context.Context, connString string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Register vector type
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if err := pgxvector.RegisterTypes(ctx, conn); err != nil {
			return fmt.Errorf("failed to register vector types: %w", err)
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{Pool: pool}, nil
}

// Close closes the connection pool.
func (db *PostgresDB) Close() {
	db.Pool.Close()
}

// PostgresKVStorage implements BaseKVStorage using PostgreSQL.
type PostgresKVStorage struct {
	db        *PostgresDB
	namespace string
	workspace string
	tableName string
	config    *StorageConfig
	mu        sync.RWMutex
}

// NewPostgresKVStorage creates a new PostgresKVStorage instance.
func NewPostgresKVStorage(db *PostgresDB, config *StorageConfig) *PostgresKVStorage {
	return &PostgresKVStorage{
		db:        db,
		namespace: config.Namespace, // e.g., "full_docs", "text_chunks"
		workspace: config.Workspace,
		tableName: "lightrag_kv_store",
		config:    config,
	}
}

func (s *PostgresKVStorage) GetNamespace() string {
	return s.namespace
}

func (s *PostgresKVStorage) GetWorkspace() string {
	return s.workspace
}

func (s *PostgresKVStorage) Initialize(ctx context.Context) error {
	// Create table if not exists
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			workspace VARCHAR(255) NOT NULL,
			namespace VARCHAR(255) NOT NULL,
			id VARCHAR(1024) NOT NULL,
			data JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (workspace, namespace, id)
		);
		CREATE INDEX IF NOT EXISTS idx_%s_namespace ON %s(workspace, namespace);
	`, s.tableName, s.tableName, s.tableName)

	_, err := s.db.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to initialize table %s: %w", s.tableName, err)
	}
	return nil
}

func (s *PostgresKVStorage) Finalize(ctx context.Context) error {
	// No specific finalization needed for Postgres
	return nil
}

func (s *PostgresKVStorage) IndexDoneCallback(ctx context.Context) error {
	// No specific callback needed for Postgres
	return nil
}

func (s *PostgresKVStorage) Drop(ctx context.Context) (map[string]string, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE workspace = $1 AND namespace = $2", s.tableName)
	_, err := s.db.Pool.Exec(ctx, query, s.workspace, s.namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to drop data: %w", err)
	}
	return map[string]string{"status": "success", "message": "data dropped"}, nil
}

func (s *PostgresKVStorage) GetByID(ctx context.Context, id string) (map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT data FROM %s WHERE workspace = $1 AND namespace = $2 AND id = $3", s.tableName)
	var data []byte
	err := s.db.Pool.QueryRow(ctx, query, s.workspace, s.namespace, id).Scan(&data)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Return nil if not found
		}
		return nil, fmt.Errorf("failed to get by id: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}
	return result, nil
}

func (s *PostgresKVStorage) GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error) {
	if len(ids) == 0 {
		return []map[string]interface{}{}, nil
	}

	query := fmt.Sprintf("SELECT data FROM %s WHERE workspace = $1 AND namespace = $2 AND id = ANY($3)", s.tableName)
	rows, err := s.db.Pool.Query(ctx, query, s.workspace, s.namespace, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get by ids: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *PostgresKVStorage) FilterKeys(ctx context.Context, keys map[string]struct{}) (map[string]struct{}, error) {
	if len(keys) == 0 {
		return map[string]struct{}{}, nil
	}

	// Extract keys to slice for query
	keysSlice := make([]string, 0, len(keys))
	for k := range keys {
		keysSlice = append(keysSlice, k)
	}

	// Find keys that exist
	query := fmt.Sprintf("SELECT id FROM %s WHERE workspace = $1 AND namespace = $2 AND id = ANY($3)", s.tableName)
	rows, err := s.db.Pool.Query(ctx, query, s.workspace, s.namespace, keysSlice)
	if err != nil {
		return nil, fmt.Errorf("failed to filter keys: %w", err)
	}
	defer rows.Close()

	existingKeys := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan id: %w", err)
		}
		existingKeys[id] = true
	}

	// Return keys that are NOT in existingKeys
	nonExistent := make(map[string]struct{})
	for key := range keys {
		if !existingKeys[key] {
			nonExistent[key] = struct{}{}
		}
	}
	return nonExistent, nil
}

func (s *PostgresKVStorage) Upsert(ctx context.Context, data map[string]map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}

	// Begin transaction
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := fmt.Sprintf(`
		INSERT INTO %s (workspace, namespace, id, data, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (workspace, namespace, id)
		DO UPDATE SET data = EXCLUDED.data, updated_at = CURRENT_TIMESTAMP
	`, s.tableName)

	batch := &pgx.Batch{}
	for id, item := range data {
		jsonData, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal data for id %s: %w", id, err)
		}
		batch.Queue(query, s.workspace, s.namespace, id, jsonData)
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

func (s *PostgresKVStorage) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE workspace = $1 AND namespace = $2 AND id = ANY($3)", s.tableName)
	_, err := s.db.Pool.Exec(ctx, query, s.workspace, s.namespace, ids)
	if err != nil {
		return fmt.Errorf("failed to delete ids: %w", err)
	}
	return nil
}

func (s *PostgresKVStorage) IsEmpty(ctx context.Context) (bool, error) {
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE workspace = $1 AND namespace = $2 LIMIT 1", s.tableName)
	var exists int
	err := s.db.Pool.QueryRow(ctx, query, s.workspace, s.namespace).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return true, nil
		}
		return false, fmt.Errorf("failed to check if empty: %w", err)
	}
	return false, nil
}

func (s *PostgresKVStorage) GetAll(ctx context.Context) (map[string]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT id, data FROM %s WHERE workspace = $1 AND namespace = $2", s.tableName)
	rows, err := s.db.Pool.Query(ctx, query, s.workspace, s.namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get all: %w", err)
	}
	defer rows.Close()

	results := make(map[string]map[string]interface{})
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal data: %w", err)
		}
		results[id] = result
	}
	return results, nil
}
