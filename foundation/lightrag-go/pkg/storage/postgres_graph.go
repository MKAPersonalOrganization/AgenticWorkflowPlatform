package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lightrag/lightrag-go/pkg/core/base"
)

// PostgresGraphStorage implements BaseGraphStorage using PostgreSQL + Apache AGE.
type PostgresGraphStorage struct {
	db        *PostgresDB
	namespace string
	workspace string
	graphName string
	config    *StorageConfig
}

// NewPostgresGraphStorage creates a new PostgresGraphStorage instance.
func NewPostgresGraphStorage(db *PostgresDB, config *StorageConfig) *PostgresGraphStorage {
	return &PostgresGraphStorage{
		db:        db,
		namespace: config.Namespace,
		workspace: config.Workspace,
		graphName: "lightrag_graph", // Shared graph, distinguishing by workspace property
		config:    config,
	}
}

func (s *PostgresGraphStorage) GetNamespace() string {
	return s.namespace
}

func (s *PostgresGraphStorage) GetWorkspace() string {
	return s.workspace
}

func (s *PostgresGraphStorage) Initialize(ctx context.Context) error {
	// Initialize AGE extension and graph

	// Create extension if not exists
	_, err := s.db.Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS age;")
	if err != nil {
		return fmt.Errorf("failed to create age extension: %w", err)
	}

	// Load AGE path
	_, err = s.db.Pool.Exec(ctx, "LOAD 'age';")
	if err != nil {
		return fmt.Errorf("failed to load age extension: %w", err)
	}

	// Set search path
	_, err = s.db.Pool.Exec(ctx, "SET search_path = ag_catalog, \"$user\", public;")
	if err != nil {
		return fmt.Errorf("failed to set search_path: %w", err)
	}

	// Create graph if not exists
	// SELECT create_graph('lightrag_graph');
	// query_graph exists checks if graph exists.
	var exists bool
	err = s.db.Pool.QueryRow(ctx, "SELECT count(*) > 0 FROM ag_catalog.ag_graph WHERE name = $1", s.graphName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check graph existence: %w", err)
	}

	if !exists {
		_, err := s.db.Pool.Exec(ctx, "SELECT create_graph($1);", s.graphName)
		if err != nil {
			return fmt.Errorf("failed to create graph %s: %w", s.graphName, err)
		}
	}

	return nil
}

func (s *PostgresGraphStorage) Finalize(ctx context.Context) error {
	return nil
}

func (s *PostgresGraphStorage) IndexDoneCallback(ctx context.Context) error {
	return nil
}

func (s *PostgresGraphStorage) Drop(ctx context.Context) (map[string]string, error) {
	cypher := fmt.Sprintf(`
		MATCH (n)
		WHERE n.workspace = '%s'
		DETACH DELETE n
	`, s.workspace)

	err := s.execCypher(ctx, cypher)
	if err != nil {
		return nil, fmt.Errorf("failed to drop workspace data: %w", err)
	}
	return map[string]string{"status": "success"}, nil
}

// execCypher executes a cypher command.
// Uses SQL: SELECT * FROM cypher('graph_name', $$ ... $$) as (a agtype);
func (s *PostgresGraphStorage) execCypher(ctx context.Context, cypherQuery string) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "LOAD 'age'; SET search_path = ag_catalog, \"$user\", public;"); err != nil {
		return err
	}

	// For void queries (CREATE, MERGE, SET, DELETE), we calculate "SELECT * FROM cypher(...) as (v agtype)" and discard result
	wrappedQuery := fmt.Sprintf("SELECT * FROM cypher('%s', $$ %s $$) as (v agtype);", s.graphName, cypherQuery)
	_, err = tx.Exec(ctx, wrappedQuery)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// queryCypher executes cypher and returns agtype results as string (JSON).
func (s *PostgresGraphStorage) queryCypher(ctx context.Context, cypherQuery string) ([]string, error) {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "LOAD 'age'; SET search_path = ag_catalog, \"$user\", public;"); err != nil {
		return nil, err
	}

	wrappedQuery := fmt.Sprintf("SELECT * FROM cypher('%s', $$ %s $$) as (v agtype);", s.graphName, cypherQuery)
	rows, err := tx.Query(ctx, wrappedQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var v interface{}
		// "SELECT * FROM cypher(...) as (v agtype)"
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		// Convert to string
		resJSON, _ := json.Marshal(v)
		results = append(results, string(resJSON))
	}

	return results, tx.Commit(ctx)
}

func (s *PostgresGraphStorage) HasNode(ctx context.Context, nodeID string) (bool, error) {
	cypher := fmt.Sprintf("MATCH (n {id: '%s', workspace: '%s'}) RETURN n", escape(nodeID), s.workspace)
	results, err := s.queryCypher(ctx, cypher)
	if err != nil {
		return false, err
	}
	return len(results) > 0, nil
}

func (s *PostgresGraphStorage) HasEdge(ctx context.Context, sourceNodeID, targetNodeID string) (bool, error) {
	cypher := fmt.Sprintf("MATCH (a {id: '%s', workspace: '%s'})-[e]->(b {id: '%s', workspace: '%s'}) RETURN e",
		escape(sourceNodeID), s.workspace, escape(targetNodeID), s.workspace)
	results, err := s.queryCypher(ctx, cypher)
	if err != nil {
		return false, err
	}
	return len(results) > 0, nil
}

func (s *PostgresGraphStorage) NodeDegree(ctx context.Context, nodeID string) (int, error) {
	// Simple query to count edges.
	cypher := fmt.Sprintf("MATCH (n {id: '%s', workspace: '%s'})-[e]-() RETURN count(e)", escape(nodeID), s.workspace)
	results, err := s.queryCypher(ctx, cypher)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	// Result handling - implement proper parsing later if needed
	return 0, nil
}

func (s *PostgresGraphStorage) EdgeDegree(ctx context.Context, srcID, tgtID string) (int, error) {
	return 0, nil
}

func (s *PostgresGraphStorage) GetNode(ctx context.Context, nodeID string) (map[string]string, error) {
	cypher := fmt.Sprintf("MATCH (n {id: '%s', workspace: '%s'}) RETURN properties(n)", escape(nodeID), s.workspace)
	results, err := s.queryCypher(ctx, cypher)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	// Parse properties from results[0]
	return map[string]string{}, nil
}

func (s *PostgresGraphStorage) GetEdge(ctx context.Context, sourceNodeID, targetNodeID string) (map[string]string, error) {
	return nil, nil
}

func (s *PostgresGraphStorage) GetNodeEdges(ctx context.Context, sourceNodeID string) ([][2]string, error) {
	return nil, nil
}

func (s *PostgresGraphStorage) GetNodesBatch(ctx context.Context, nodeIDs []string) (map[string]map[string]string, error) {
	return map[string]map[string]string{}, nil
}

func (s *PostgresGraphStorage) UpsertNode(ctx context.Context, nodeID string, nodeData map[string]string) error {
	// properties map
	props := make(map[string]interface{})
	for k, v := range nodeData {
		props[k] = v
	}
	props["id"] = nodeID
	props["workspace"] = s.workspace

	propsString := string(propsJSON(props))

	// Cypher MERGE using Generic 'Entity' label for simplicity
	cypher := fmt.Sprintf(`
		MERGE (n:Entity {id: '%s', workspace: '%s'})
		SET n += %s
	`, escape(nodeID), s.workspace, propsString)

	return s.execCypher(ctx, cypher)
}

func (s *PostgresGraphStorage) UpsertEdge(ctx context.Context, sourceNodeID, targetNodeID string, edgeData map[string]string) error {
	props := make(map[string]interface{})
	for k, v := range edgeData {
		props[k] = v
	}
	props["workspace"] = s.workspace
	propsString := string(propsJSON(props))

	// MATCH source and target first
	cypher := fmt.Sprintf(`
		MATCH (a:Entity {id: '%s', workspace: '%s'}), (b:Entity {id: '%s', workspace: '%s'})
		MERGE (a)-[e:RELATION]->(b)
		SET e += %s
	`, escape(sourceNodeID), s.workspace, escape(targetNodeID), s.workspace, propsString)

	return s.execCypher(ctx, cypher)
}

func (s *PostgresGraphStorage) DeleteNode(ctx context.Context, nodeID string) error {
	cypher := fmt.Sprintf(`
		MATCH (n:Entity {id: '%s', workspace: '%s'})
		DETACH DELETE n
	`, escape(nodeID), s.workspace)
	return s.execCypher(ctx, cypher)
}

func (s *PostgresGraphStorage) DeleteEdge(ctx context.Context, sourceNodeID, targetNodeID string) error {
	cypher := fmt.Sprintf(`
		MATCH (a:Entity {id: '%s', workspace: '%s'})-[e:RELATION]->(b:Entity {id: '%s', workspace: '%s'})
		DELETE e
	`, escape(sourceNodeID), s.workspace, escape(targetNodeID), s.workspace)
	return s.execCypher(ctx, cypher)
}

func (s *PostgresGraphStorage) GetKnowledgeGraph(ctx context.Context, nodeLabel string, maxDepth, maxNodes int) (*base.KnowledgeGraph, error) {
	// Simple implementation
	return &base.KnowledgeGraph{
		Nodes: []base.KnowledgeGraphNode{},
		Edges: []base.KnowledgeGraphEdge{},
	}, nil
}

// Helpers
func escape(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func propsJSON(m map[string]interface{}) []byte {
	b, _ := json.Marshal(m)
	return b
}

// Missing methods implementation to satisfy interface
func (s *PostgresGraphStorage) NodeDegreesBatch(ctx context.Context, nodeIDs []string) (map[string]int, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *PostgresGraphStorage) EdgeDegreesBatch(ctx context.Context, edgePairs [][2]string) (map[[2]string]int, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *PostgresGraphStorage) GetEdgesBatch(ctx context.Context, pairs []map[string]string) (map[[2]string]map[string]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *PostgresGraphStorage) GetNodesEdgesBatch(ctx context.Context, nodeIDs []string) (map[string][][2]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *PostgresGraphStorage) RemoveNodes(ctx context.Context, nodes []string) error {
	return nil
}

func (s *PostgresGraphStorage) RemoveEdges(ctx context.Context, edges [][2]string) error {
	return nil
}

func (s *PostgresGraphStorage) GetAllLabels(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (s *PostgresGraphStorage) GetAllNodes(ctx context.Context) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (s *PostgresGraphStorage) GetAllEdges(ctx context.Context) ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

func (s *PostgresGraphStorage) GetPopularLabels(ctx context.Context, limit int) ([]string, error) {
	return []string{}, nil
}

func (s *PostgresGraphStorage) SearchLabels(ctx context.Context, query string, limit int) ([]string, error) {
	return []string{}, nil
}
